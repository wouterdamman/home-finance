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

function isNumberArray(v: unknown): v is number[] {
  return Array.isArray(v) && v.every((x) => typeof x === 'number')
}

function normalizeChartKind(v: unknown): ChartKind {
  return v === 'bar' ? 'bar' : 'line'
}

// Validates (and where possible migrates) a persisted widget config's shape.
// Storage-schema changes have broken this before — `categoryChart` moved from
// a single `categoryId: number` to `categoryIds: number[]` under the same
// storage key, and an unvalidated old config crashed the page with
// `config.categoryIds.map is not a function`. Returns null for configs that
// can't be salvaged, so the widget gets dropped instead of crashing the page.
function sanitizeConfig(config: unknown): WidgetConfig | null {
  if (typeof config !== 'object' || config === null || !('type' in config)) return null
  const c = config as Record<string, unknown>
  switch (c.type) {
    case 'kpi':
      if (c.metric === 'income' || c.metric === 'expenses' || c.metric === 'surplus' || c.metric === 'yearsTracked') {
        return { type: 'kpi', metric: c.metric }
      }
      return null
    case 'categoryChart': {
      let categoryIds: number[] | null = null
      if (isNumberArray(c.categoryIds) && c.categoryIds.length > 0) {
        categoryIds = c.categoryIds
      } else if (typeof c.categoryId === 'number') {
        // Pre-multi-select shape: migrate the single id into the array form.
        categoryIds = [c.categoryId]
      }
      if (categoryIds == null) return null
      return { type: 'categoryChart', categoryIds, chartKind: normalizeChartKind(c.chartKind) }
    }
    case 'monthCompare':
      if (typeof c.year === 'number' && isNumberArray(c.months) && c.months.length > 0) {
        return { type: 'monthCompare', year: c.year, months: c.months, chartKind: normalizeChartKind(c.chartKind) }
      }
      return null
    case 'allTimeTrend':
      if (typeof c.fromYear === 'number' && typeof c.toYear === 'number') {
        return { type: 'allTimeTrend', fromYear: c.fromYear, toYear: c.toYear, chartKind: normalizeChartKind(c.chartKind) }
      }
      return null
    case 'monthAcrossYears':
      if (typeof c.month === 'number' && isNumberArray(c.years) && c.years.length > 0) {
        return { type: 'monthAcrossYears', month: c.month, years: c.years, chartKind: normalizeChartKind(c.chartKind) }
      }
      return null
    default:
      return null
  }
}

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
    // `yearCompare`) or whose config shape doesn't match the current schema
    // (e.g. pre-multi-select `categoryChart`) are dropped/migrated rather
    // than crashing the render — see sanitizeConfig.
    const out: Widget[] = []
    for (const w of parsed.widgets as Record<string, unknown>[]) {
      const config = sanitizeConfig(w.config)
      if (config == null || typeof w.id !== 'string') continue
      out.push({
        id: w.id,
        visible: typeof w.visible === 'boolean' ? w.visible : true,
        width: (typeof w.width === 'number' ? w.width : 1) as WidgetWidth,
        height: (typeof w.height === 'number' ? w.height : defaultHeight(config)) as WidgetHeight,
        config,
      })
    }
    return out
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
