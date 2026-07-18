export interface TrendsFilter {
  year: number
}

export const MAX_COMPARE_YEARS = 4

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
