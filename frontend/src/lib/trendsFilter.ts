export type TrendsFilter =
  | { mode: 'single'; year: number }
  | { mode: 'compare'; years: number[] }

export const MAX_COMPARE_YEARS = 4

// Fixed slot-per-position color mapping — never cycled. Position = index
// within the selected years, sorted ascending, so "oldest selected year" is
// always teal, "2nd oldest" is always blue, etc. across every widget on the
// page. Capped at MAX_COMPARE_YEARS, so this array never needs a 5th slot.
export const YEAR_COLORS = ['teal.6', 'blue.6', 'grape.6', 'orange.6']

const STORAGE_KEY = 'trends-year-filter'

export function loadTrendsFilter(availableYears: number[]): TrendsFilter {
  const fallbackYear = availableYears[availableYears.length - 1] ?? new Date().getFullYear()
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { mode: 'single', year: fallbackYear }
    const parsed = JSON.parse(raw) as TrendsFilter
    if (parsed.mode === 'single' && availableYears.includes(parsed.year)) {
      return parsed
    }
    if (parsed.mode === 'compare') {
      const years = parsed.years.filter((y) => availableYears.includes(y)).slice(0, MAX_COMPARE_YEARS)
      if (years.length >= 2) return { mode: 'compare', years }
    }
  } catch {
    // fall through to default
  }
  return { mode: 'single', year: fallbackYear }
}

export function saveTrendsFilter(filter: TrendsFilter) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(filter))
}

export function sortedCompareYears(filter: TrendsFilter): number[] {
  return filter.mode === 'compare' ? [...filter.years].sort((a, b) => a - b) : []
}
