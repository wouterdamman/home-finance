export interface TrendsFilter {
  year: number
}

export const MAX_COMPARE_YEARS = 4

// Fixed slot-per-position color mapping — never cycled. Used by widgets that
// compare a handful of years/months side by side (e.g. MonthCompareWidget's
// month coloring) so identity stays consistent without ever generating a hue.
export const YEAR_COLORS = ['teal.6', 'blue.6', 'grape.6', 'orange.6']

const STORAGE_KEY = 'trends-year-filter'

export function loadTrendsFilter(availableYears: number[]): TrendsFilter {
  const fallbackYear = availableYears[availableYears.length - 1] ?? new Date().getFullYear()
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { year: fallbackYear }
    const parsed = JSON.parse(raw) as { year?: number }
    if (parsed.year != null && availableYears.includes(parsed.year)) {
      return { year: parsed.year }
    }
  } catch {
    // fall through to default
  }
  return { year: fallbackYear }
}

export function saveTrendsFilter(filter: TrendsFilter) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(filter))
}
