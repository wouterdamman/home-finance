package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/wouterdamman/home-finance/internal/auth"
	"github.com/wouterdamman/home-finance/internal/config"
)

type Server struct {
	cfg  *config.Config
	pool *pgxpool.Pool
	sm   *scs.SessionManager
	oidc *auth.Provider
	s3   *minio.Client
}

// newS3Client returns nil (not an error) when S3 isn't configured — avatar
// upload/proxy handlers check for that and respond with a clear error
// instead of the app failing to start without object storage.
func newS3Client(cfg *config.Config) *minio.Client {
	if !cfg.S3Configured() {
		return nil
	}
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
		Region: cfg.S3Region,
	})
	if err != nil {
		slog.Error("s3 client init failed", "err", err)
		return nil
	}
	return client
}

func NewServer(cfg *config.Config, pool *pgxpool.Pool, sm *scs.SessionManager, oidcProvider *auth.Provider) http.Handler {
	s := &Server{cfg: cfg, pool: pool, sm: sm, oidc: oidcProvider, s3: newS3Client(cfg)}
	passwordLimiter := newPasswordRateLimiter()
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(sm.LoadAndSave)

	// healthz is pure liveness (process alive) — no DB check, so a flaky DB
	// doesn't cause k8s to kill and restart an otherwise-healthy pod.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	// readyz is readiness — pings the pool so k8s stops routing traffic here
	// when the DB is unreachable, without restarting the pod.
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			Error(w, http.StatusServiceUnavailable, "db_unreachable", err.Error())
			return
		}
		w.WriteHeader(200)
	})

	r.Get("/auth/login", s.handleAuthLogin)
	r.Get("/auth/callback", s.handleAuthCallback)
	r.Post("/auth/logout", s.handleAuthLogout)

	requireAuth := auth.Require(sm, cfg.DevFakeAuth)
	requireAdmin := auth.RequireAdmin(pool)

	r.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireCSRF)
		r.With(requireAuth).Get("/me", s.handleMe)
		r.With(requireAuth).Patch("/me", s.handleUpdateMe)
		r.With(requireAuth).Post("/me/avatar", s.handleUploadAvatar)
		r.With(requireAuth).Get("/users/{id}/avatar", s.handleGetAvatar)
		r.With(requireAuth, requireAdmin).Get("/docs", s.handleAPIDocs)
		r.With(requireAuth, requireAdmin).Get("/openapi.yaml", s.handleOpenAPISpec)
		r.With(requireAuth, requireAdmin).Get("/users", s.handleListUsers)
		r.With(requireAuth, requireAdmin).Patch("/users/{id}/role", s.handleUpdateUserRole)

		r.With(requireAuth).Group(func(r chi.Router) {
			// Periods
			r.Get("/periods", s.handleListPeriods)
			r.Post("/periods", s.handleCreatePeriod)
			r.Get("/periods/{id}/overview", s.handleGetPeriodOverview)
			r.With(requireAdmin).Post("/periods/{id}/close", s.handleClosePeriod)
			r.With(requireAdmin).Post("/periods/{id}/reopen", s.handleReopenPeriod)
			r.With(requireAdmin, passwordLimiter.middleware).Delete("/periods/{id}", s.handleDeletePeriod)

			// Audit log
			r.With(requireAdmin).Get("/audit-log", s.handleListAuditLog)

			// Income entries
			r.Post("/periods/{id}/incomes", s.handleCreateIncomeEntry)
			r.Put("/incomes/{id}", s.handleUpdateIncomeEntry)
			r.Delete("/incomes/{id}", s.handleDeleteIncomeEntry)

			// Budget lines
			r.Post("/periods/{id}/budget-lines", s.handleCreateBudgetLine)
			r.Put("/budget-lines/{id}", s.handleUpdateBudgetLine)
			r.Delete("/budget-lines/{id}", s.handleDeleteBudgetLine)

			// Transactions
			r.Get("/periods/{id}/transactions", s.handleListTransactions)
			r.Post("/periods/{id}/transactions", s.handleCreateTransaction)
			r.Put("/transactions/{id}", s.handleUpdateTransaction)
			r.Delete("/transactions/{id}", s.handleDeleteTransaction)

			// Splits
			r.Put("/periods/{id}/splits", s.handleReplaceSplits)

			// Categories
			r.Get("/categories", s.handleListCategories)
			r.With(requireAdmin).Post("/categories", s.handleCreateCategory)
			r.With(requireAdmin).Put("/categories/order", s.handleReorderCategories)
			r.With(requireAdmin).Put("/categories/{id}", s.handleUpdateCategory)
			r.With(requireAdmin).Post("/categories/{id}/archive", s.handleArchiveCategory)

			// Income sources
			r.Get("/income-sources", s.handleListIncomeSources)
			r.With(requireAdmin).Post("/income-sources", s.handleCreateIncomeSource)
			r.With(requireAdmin).Put("/income-sources/{id}", s.handleUpdateIncomeSource)
			r.With(requireAdmin).Post("/income-sources/{id}/archive", s.handleArchiveIncomeSource)

			// Pots
			r.Get("/pots/balances", s.handleGetPotBalances)
			r.Get("/pots", s.handleListPots)
			r.With(requireAdmin).Post("/pots", s.handleCreatePot)
			r.With(requireAdmin).Put("/pots/{id}", s.handleUpdatePot)
			r.With(requireAdmin).Post("/pots/{id}/archive", s.handleArchivePot)
			r.Get("/pots/{id}/ledger", s.handleGetPotLedger)
			r.Post("/pots/{id}/entries", s.handleCreatePotEntry)
			r.Delete("/pot-entries/{id}", s.handleDeletePotEntry)

			// Years
			r.Get("/years", s.handleListYears)
			r.With(requireAdmin).Post("/years", s.handleCreateYear)
			r.Get("/years/{year}/summary", s.handleYearSummary)
			r.With(requireAdmin, passwordLimiter.middleware).Post("/years/{year}/lock", s.handleLockYear)
			r.With(requireAdmin, passwordLimiter.middleware).Post("/years/{year}/unlock", s.handleUnlockYear)

			// Export
			r.Get("/export/years/{year}", s.handleExportYear)

			// Import
			r.With(passwordLimiter.middleware).Post("/import/xlsx", s.handleImportXLSX)
		})
	})

	if cfg.StaticDir != "" {
		r.NotFound(SPAHandler(cfg.StaticDir).ServeHTTP)
	}
	return r
}
