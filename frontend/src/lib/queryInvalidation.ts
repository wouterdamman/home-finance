import type { QueryClient } from '@tanstack/react-query'

// Editing a period's income/expense rows changes every aggregate derived from
// it, and none of those caches are keyed by periodId — without this they keep
// serving pre-edit numbers for the whole `staleTime` window (main.tsx: 30s).
export function invalidatePeriodAggregates(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: ['year-summary'] })
  qc.invalidateQueries({ queryKey: ['trends-category-totals'] })
  qc.invalidateQueries({ queryKey: ['trends-years'] })
  qc.invalidateQueries({ queryKey: ['trends-monthly-totals'] })
  qc.invalidateQueries({ queryKey: ['trends-income-sources'] })
}

// Closing/reopening writes allocation ledger rows and a carryover entry, so
// every pot balance and every pot's detail ledger changes — the ['pots']
// registry key alone only covers name/kind/target.
export function invalidatePotLedgerQueries(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: ['pot-balances'] })
  qc.invalidateQueries({ queryKey: ['pot-ledger'] })
  qc.invalidateQueries({ queryKey: ['trends-pot-balances'] })
}
