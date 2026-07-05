package httpapi

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TheIronRock95/home-finance/internal/auth"
	"github.com/TheIronRock95/home-finance/internal/config"
)

type Server struct {
	cfg  *config.Config
	pool *pgxpool.Pool
	sm   *scs.SessionManager
	oidc *auth.Provider
}

func NewServer(cfg *config.Config, pool *pgxpool.Pool, sm *scs.SessionManager, oidcProvider *auth.Provider) http.Handler {
	s := &Server{cfg: cfg, pool: pool, sm: sm, oidc: oidcProvider}
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(sm.LoadAndSave)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	r.Get("/auth/login", s.handleAuthLogin)
	r.Get("/auth/callback", s.handleAuthCallback)
	r.Post("/auth/logout", s.handleAuthLogout)

	requireAuth := auth.Require(sm, cfg.DevFakeAuth)

	r.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireCSRF)
		r.With(requireAuth).Get("/me", s.handleMe)

		r.With(requireAuth).Group(func(r chi.Router) {
			// Periods
			r.Get("/periods", s.handleListPeriods)
			r.Post("/periods", s.handleCreatePeriod)
			r.Get("/periods/{id}/overview", s.handleGetPeriodOverview)
			r.Post("/periods/{id}/close", s.handleClosePeriod)
			r.Post("/periods/{id}/reopen", s.handleReopenPeriod)
			r.Delete("/periods/{id}", s.handleDeletePeriod)

			// Audit log
			r.Get("/audit-log", s.handleListAuditLog)

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
			r.Post("/categories", s.handleCreateCategory)
			r.Put("/categories/order", s.handleReorderCategories)
			r.Put("/categories/{id}", s.handleUpdateCategory)
			r.Post("/categories/{id}/archive", s.handleArchiveCategory)

			// Income sources
			r.Get("/income-sources", s.handleListIncomeSources)
			r.Post("/income-sources", s.handleCreateIncomeSource)
			r.Put("/income-sources/{id}", s.handleUpdateIncomeSource)
			r.Post("/income-sources/{id}/archive", s.handleArchiveIncomeSource)

			// Pots
			r.Get("/pots/balances", s.handleGetPotBalances)
			r.Get("/pots", s.handleListPots)
			r.Post("/pots", s.handleCreatePot)
			r.Put("/pots/{id}", s.handleUpdatePot)
			r.Post("/pots/{id}/archive", s.handleArchivePot)
			r.Get("/pots/{id}/ledger", s.handleGetPotLedger)
			r.Post("/pots/{id}/entries", s.handleCreatePotEntry)

			// Year summary
			r.Get("/years/{year}/summary", s.handleYearSummary)
			r.Post("/years/{year}/lock", s.handleLockYear)
			r.Post("/years/{year}/unlock", s.handleUnlockYear)
		})
	})

	if cfg.StaticDir != "" {
		r.NotFound(SPAHandler(cfg.StaticDir).ServeHTTP)
	}
	return r
}
