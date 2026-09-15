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

	"github.com/wouterdamman/home-finance/internal/auth"
	"github.com/wouterdamman/home-finance/internal/avatarstorage"
	"github.com/wouterdamman/home-finance/internal/config"
)

type Server struct {
	cfg           *config.Config
	pool          *pgxpool.Pool
	sm            *scs.SessionManager
	oidc          *auth.Provider
	avatarStorage avatarstorage.Storage
}

func NewServer(cfg *config.Config, pool *pgxpool.Pool, sm *scs.SessionManager, oidcProvider *auth.Provider) http.Handler {
	s := &Server{cfg: cfg, pool: pool, sm: sm, oidc: oidcProvider, avatarStorage: avatarstorage.NewPostgres(pool)}
	if cfg.DevFakeAuth {
		// Callers that hit the API directly (integration tests, curl) never
		// go through /auth/login, so the dev admin row must exist up front —
		// otherwise requireAdmin's DB lookup for user id=1 finds nothing and
		// every admin-gated route 403s.
		s.ensureDevUser(context.Background())
	}
	authFlowLimiter := newAuthFlowLimiter()
	r := chi.NewRouter()
	// No middleware.RealIP: chi's own source deprecates it as spoofable (it
	// rewrites RemoteAddr from caller-supplied X-Forwarded-For / X-Real-IP /
	// True-Client-IP with no trusted-proxy list). The rate limiter keys on the
	// session user id instead, so nothing here needs a client IP. Adding a
	// limiter to an unauthenticated route would need a trusted-proxy-aware
	// client IP first — not this.
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(bodyLimit)
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
			slog.Warn("readyz db ping failed", "err", err)
			Error(w, http.StatusServiceUnavailable, "db_unreachable", "database unreachable")
			return
		}
		w.WriteHeader(200)
	})

	r.Get("/auth/login", s.handleAuthLogin)
	r.Get("/auth/callback", s.handleAuthCallback)
	r.With(auth.RequireCSRF).Post("/auth/logout", s.handleAuthLogout)

	requireAuth := auth.Require(sm, cfg.DevFakeAuth)
	requireAdmin := auth.RequireAdmin(pool)

	r.With(requireAuth, authFlowLimiter.middleware).Get("/auth/reauth", s.handleAuthReauth)

	r.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireCSRF)
		r.NotFound(apiNotFound)
		r.MethodNotAllowed(apiMethodNotAllowed)
		r.With(requireAuth).Get("/me", s.handleMe)
		r.With(requireAuth).Get("/reauth-status", s.handleReauthStatus)
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
			r.With(requireAdmin).Delete("/periods/{id}", s.handleDeletePeriod)

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
			r.Get("/periods/{id}/income-transactions", s.handleListIncomeTransactions)
			r.Post("/periods/{id}/income-transactions", s.handleCreateIncomeTransaction)
			r.Put("/income-transactions/{id}", s.handleUpdateIncomeTransaction)
			r.Delete("/income-transactions/{id}", s.handleDeleteIncomeTransaction)

			// Splits
			r.Put("/periods/{id}/splits", s.handleReplaceSplits)

			// Categories
			r.Get("/categories", s.handleListCategories)
			r.With(requireAdmin).Post("/categories", s.handleCreateCategory)
			r.With(requireAdmin).Put("/categories/order", s.handleReorderCategories)
			r.With(requireAdmin).Put("/categories/{id}", s.handleUpdateCategory)
			r.With(requireAdmin).Post("/categories/{id}/archive", s.handleArchiveCategory)
			r.Get("/categories/{id}/transaction-descriptions", s.handleListCategoryTransactionDescriptions)
			r.Get("/categories/{id}/description-presets", s.handleListCategoryDescriptionPresets)
			r.With(requireAdmin).Post("/categories/{id}/description-presets", s.handleCreateCategoryDescriptionPreset)
			r.With(requireAdmin).Delete("/category-description-presets/{id}", s.handleDeleteCategoryDescriptionPreset)

			// Category aliases (import Details-table header -> parent category)
			r.Get("/category-aliases", s.handleListCategoryAliases)
			r.With(requireAdmin).Post("/category-aliases", s.handleCreateCategoryAlias)
			r.With(requireAdmin).Delete("/category-aliases/{id}", s.handleDeleteCategoryAlias)

			// Income sources
			r.Get("/income-sources", s.handleListIncomeSources)
			r.With(requireAdmin).Post("/income-sources", s.handleCreateIncomeSource)
			r.With(requireAdmin).Put("/income-sources/{id}", s.handleUpdateIncomeSource)
			r.With(requireAdmin).Post("/income-sources/{id}/archive", s.handleArchiveIncomeSource)
			r.With(requireAdmin).Delete("/income-sources/{id}", s.handleDeleteIncomeSource)
			r.Get("/income-sources/{id}/transaction-descriptions", s.handleListIncomeSourceTransactionDescriptions)
			r.Get("/income-sources/{id}/description-presets", s.handleListIncomeSourceDescriptionPresets)
			r.With(requireAdmin).Post("/income-sources/{id}/description-presets", s.handleCreateIncomeSourceDescriptionPreset)
			r.With(requireAdmin).Delete("/income-source-description-presets/{id}", s.handleDeleteIncomeSourceDescriptionPreset)

			// Pots
			r.Get("/pots/balances", s.handleGetPotBalances)
			r.Get("/pots", s.handleListPots)
			r.With(requireAdmin).Post("/pots", s.handleCreatePot)
			r.With(requireAdmin).Put("/pots/{id}", s.handleUpdatePot)
			r.With(requireAdmin).Post("/pots/{id}/archive", s.handleArchivePot)
			r.Get("/pots/{id}/ledger", s.handleGetPotLedger)
			r.Post("/pots/{id}/entries", s.handleCreatePotEntry)
			r.Patch("/pot-entries/{id}", s.handleUpdatePotEntry)
			r.Delete("/pot-entries/{id}", s.handleDeletePotEntry)

			// Kids savings
			r.Get("/kids/balances", s.handleGetKidBalances)
			r.Get("/kids", s.handleListKids)
			r.With(requireAdmin).Patch("/kids/{id}/reported-balance", s.handleUpdateKidReportedBalance)
			r.Get("/kids/{id}/ledger", s.handleGetKidLedger)
			r.Post("/kids/{id}/entries", s.handleCreateKidLedgerEntry)
			r.Patch("/kid-entries/{id}", s.handleUpdateKidLedgerEntry)
			r.Delete("/kid-entries/{id}", s.handleDeleteKidLedgerEntry)

			// Years
			r.Get("/years", s.handleListYears)
			r.With(requireAdmin).Post("/years", s.handleCreateYear)
			r.Get("/years/{year}/summary", s.handleYearSummary)
			r.Get("/trends/years", s.handleTrendsYears)
			r.Get("/trends/category-totals", s.handleTrendsCategoryTotals)
			r.Get("/trends/monthly-totals", s.handleTrendsMonthlyTotals)
			r.With(requireAdmin).Post("/years/{year}/lock", s.handleLockYear)
			r.With(requireAdmin).Post("/years/{year}/unlock", s.handleUnlockYear)

			// Export
			r.With(requireAdmin).Get("/export/years/{year}", s.handleExportYear)

			// Import
			r.With(requireAdmin).Post("/import/xlsx", s.handleImportXLSX)
		})
	})

	if cfg.StaticDir != "" {
		r.NotFound(SPAHandler(cfg.StaticDir).ServeHTTP)
	}
	return r
}
