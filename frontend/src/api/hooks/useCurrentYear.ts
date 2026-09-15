import { useYears } from './usePeriods'

function resolveYear(years: number[] | undefined): number {
  const thisYear = new Date().getFullYear()
  if (!years || years.length === 0) return thisYear
  if (years.includes(thisYear)) return thisYear
  const priorYears = years.filter((y) => y <= thisYear)
  if (priorYears.length > 0) return Math.max(...priorYears)
  return Math.max(...years)
}

// Resolves which year the "Home" nav destination should land on: the
// current calendar year if it's already registered, otherwise the closest
// registered year at or before it, otherwise whatever's registered at all
// (falls back gracefully for brand-new or future-dated installs).
export function useCurrentYear(): number {
  const { data: years } = useYears()
  return resolveYear(years)
}

// Same resolution, plus the load state — callers that navigate (rather than
// just render a link) must wait, or they redirect to the calendar-year
// fallback before the registry has arrived.
export function useCurrentYearState(): { year: number; isPending: boolean } {
  const { data: years, isPending } = useYears()
  return { year: resolveYear(years), isPending }
}
