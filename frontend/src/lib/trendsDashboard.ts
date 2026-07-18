export type ChartKind = 'line' | 'bar'

export type WidgetConfig =
  | { type: 'kpi'; metric: 'income' | 'expenses' | 'surplus' | 'yearsTracked' }
  | { type: 'categoryChart'; categoryIds: number[]; chartKind: ChartKind }
  // Self-contained: its own year + month selection, independent of the
  // page's global year filter — compares specific months within one year
  // (e.g. "why was Jan pricier than Feb") rather than years against
  // each other.
  | { type: 'monthCompare'; year: number; months: number[]; chartKind: ChartKind }
  // Continuous income/expenses/surplus per month across a year range —
  // fromYear/toYear lets it zoom to 1 year (short-term) or span several
  // (long-term), independent of the page's global year filter.
  | { type: 'allTimeTrend'; fromYear: number; toYear: number; chartKind: ChartKind }
  // Same single month, compared across up to MAX_COMPARE_YEARS years.
  | { type: 'monthAcrossYears'; month: number; years: number[]; chartKind: ChartKind }

// Size in grid units. Width = columns (desktop grid is 4 wide, see
// Trends.tsx's SimpleGrid `cols`). Height = row-units of a fixed
// `ROW_UNIT_PX` each (see Trends.tsx) — fixed row tracks are what make
// `gridAutoFlow: dense` actually pack smaller widgets into the leftover
// space next to a bigger one; auto-sized rows can't do that (a shorter
// neighbour just gets stretched with dead space, which is the bug this
// replaced).
export type WidgetWidth = 1 | 2 | 3 | 4
export type WidgetHeight = 1 | 2 | 3 | 4

export interface Widget {
  id: string
  visible: boolean
  width: WidgetWidth
  height: WidgetHeight
  config: WidgetConfig
}

const STORAGE_KEY = 'trends-dashboard-v1'

function genId(): string {
  return Math.random().toString(36).slice(2, 10)
}

function defaultHeight(config: WidgetConfig): WidgetHeight {
  if (config.type === 'kpi') return 1
  if (config.type === 'monthCompare' || config.type === 'allTimeTrend' || config.type === 'monthAcrossYears') return 3
  return 2
}

const KNOWN_WIDGET_TYPES: WidgetConfig['type'][] = ['kpi', 'categoryChart', 'monthCompare', 'allTimeTrend', 'monthAcrossYears']

// One multi-category widget (not 8 separate ones — that was the clutter
// complaint) with the biggest-spend categories, plus a recent-months
// comparison, so the empty dashboard already answers real questions instead
// of just showing totals. `topCategoryIds` is expected pre-sorted by total
// spend descending (the trends/category-totals API already sorts this way).
export function defaultWidgets(years: number[], topCategoryIds: number[], recentMonths: number[]): Widget[] {
  const fromYear = years.length > 0 ? Math.min(...years) : new Date().getFullYear()
  const toYear = years.length > 0 ? Math.max(...years) : new Date().getFullYear()
  const lastYear = years.length > 0 ? Math.max(...years) : new Date().getFullYear()
  const configs: WidgetConfig[] = [
    { type: 'kpi', metric: 'income' },
    { type: 'kpi', metric: 'expenses' },
    { type: 'kpi', metric: 'surplus' },
    { type: 'kpi', metric: 'yearsTracked' },
    { type: 'allTimeTrend', fromYear, toYear, chartKind: 'line' },
  ]
  if (topCategoryIds.length > 0) {
    configs.push({ type: 'categoryChart', categoryIds: topCategoryIds.slice(0, 4), chartKind: 'line' })
  }
  if (recentMonths.length >= 2) {
    configs.push({ type: 'monthCompare', year: lastYear, months: recentMonths.slice(-3), chartKind: 'line' })
  }
  return configs.map((config) => ({ id: genId(), visible: true, width: 1, height: defaultHeight(config), config }))
}

export function loadDashboard(): Widget[] | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as { widgets?: unknown }
    if (!Array.isArray(parsed.widgets)) return null
    // Widgets saved before `width`/`height` existed don't have the fields —
    // default them rather than letting `gridColumn/gridRow: span undefined` break.
    // Widgets whose type has since been removed (e.g. the retired
    // `yearCompare`) are dropped rather than crashing the render.
    return (parsed.widgets as Widget[])
      .filter((w) => KNOWN_WIDGET_TYPES.includes(w.config?.type))
      .map((w) => ({
        ...w,
        width: w.width ?? 1,
        height: w.height ?? defaultHeight(w.config),
      }))
  } catch {
    return null
  }
}

export function saveDashboard(widgets: Widget[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ widgets }))
}

export function newWidget(config: WidgetConfig): Widget {
  return { id: genId(), visible: true, width: 1, height: defaultHeight(config), config }
}
