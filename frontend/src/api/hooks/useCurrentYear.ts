import { useYears } from './usePeriods'

// Resolves which year the "Home" nav destination should land on: the
// current calendar year if it's already registered, otherwise the closest
// registered year at or before it, otherwise whatever's registered at all
// (falls back gracefully for brand-new or future-dated installs).
export function useCurrentYear(): number {
  const { data: years } = useYears()
  const thisYear = new Date().getFullYear()
  if (!years || years.length === 0) return thisYear
  if (years.includes(thisYear)) return thisYear
  const priorYears = years.filter((y) => y <= thisYear)
  if (priorYears.length > 0) return Math.max(...priorYears)
  return Math.max(...years)
}
