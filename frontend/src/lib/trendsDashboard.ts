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
  // Stacked bar/area of income per source per month — scoped to the page's
  // global year filter, same as categoryChart. Up to MAX_INCOME_SOURCE_SLOTS
  // sources (8 palette.categorical slots, not the 4-slot MAX_CATEGORY_SLOTS).
  | { type: 'incomeSources'; sourceIds: number[]; chartKind: ChartKind }
  // Donut of the filter year's income share per source — no config fields
  // of its own beyond type, same page-level year filter as incomeSources.
  | { type: 'incomeMix' }
  // Self-contained ranked breakdown of one itemized income source's
  // transaction descriptions (trends/descriptions?sourceId=) — independent
  // of the page's global year filter, like monthCompare/allTimeTrend.
  | { type: 'itemizedIncome'; sourceId: number }
  // Stacked area/bar of each selected pot's closing balance per month —
  // scoped to the page's global year filter, same as categoryChart/
  // incomeSources. Up to MAX_POT_SLOTS pots (8 palette.categorical slots).
  | { type: 'potBalances'; potIds: number[]; chartKind: ChartKind }
  // Signed inflow(+)/outflow(-) bars per selected pot per month, on one
  // y-axis — same page-level year filter as potBalances. No chartKind: it's
  // always bars (a signed stacked area reads badly, unlike potBalances).
  | { type: 'potFlow'; potIds: number[] }
  // Not a chart: a Progress row per pot with a savings target, showing
  // current/target, percent and a projected completion month. No config
  // fields of its own beyond type — self-contained like incomeMix, and
  // independent of the page's year filter (it always reflects "now").
  | { type: 'potTargets' }
  // Surplus/income per month as a percentage, for one self-contained year
  // (own year field, like monthCompare/allTimeTrend) — independent of the
  // page's global year filter.
  | { type: 'savingsRate'; year: number }
  // Donut of the filter year's expense share per category — no config
  // fields of its own beyond type, same page-level year filter as
  // categoryChart/incomeMix. Always includes an explicit "uncategorised"
  // slice for the gap between category-totals and the real year expense
  // total (budget lines without a category).
  | { type: 'categoryShare' }
  // Self-contained ranked breakdown of transaction descriptions
  // (trends/descriptions?categoryId=&limit=), independent of the page's
  // global year filter, like itemizedIncome. categoryId null means "all
  // categories".
  | { type: 'topDescriptions'; categoryId: number | null; limit: number }
  // Self-contained three-layer Sankey (income sources -> Income hub ->
  // expense categories, plus a surplus branch to pots that received
  // allocations) for one year — own year field, like monthCompare/
  // allTimeTrend/savingsRate.
  | { type: 'sankeyFlow'; year: number }

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
  if (
    config.type === 'monthCompare' || config.type === 'allTimeTrend' || config.type === 'monthAcrossYears'
    || config.type === 'incomeSources' || config.type === 'incomeMix' || config.type === 'itemizedIncome'
    || config.type === 'potBalances' || config.type === 'potFlow' || config.type === 'potTargets' || config.type === 'savingsRate'
    || config.type === 'categoryShare' || config.type === 'topDescriptions'
  ) return 3
  if (config.type === 'sankeyFlow') return 4
  return 2
}

function isNumberArray(v: unknown): v is number[] {
  return Array.isArray(v) && v.every((x) => typeof x === 'number')
}

// `palette.categorical` has exactly 4 slots, so a persisted config with more
// category ids would render series with an undefined color.
const MAX_CATEGORY_SLOTS = 4

// `palette.categorical` has 8 slots — incomeSources (and incomeMix's
// "real entities before folding into Other") are capped here, independent
// of MAX_CATEGORY_SLOTS which stays at 4 for categoryChart.
export const MAX_INCOME_SOURCE_SLOTS = 8

// Same 8 palette.categorical slots, independent constant because pots and
// income sources are different entity sets that could have different limits
// in the future.
export const MAX_POT_SLOTS = 8

function isMonth(v: unknown): v is number {
  return typeof v === 'number' && Number.isInteger(v) && v >= 1 && v <= 12
}

function isYear(v: unknown): v is number {
  return typeof v === 'number' && Number.isInteger(v) && v >= 1900 && v <= 9999
}

// Income source ids are >= 0 (0 is the carryover bucket, not a real row in
// `income_sources`, but a valid selectable series all the same).
function isSourceId(v: unknown): v is number {
  return typeof v === 'number' && Number.isInteger(v) && v >= 0
}

function isPotId(v: unknown): v is number {
  return typeof v === 'number' && Number.isInteger(v) && v >= 0
}

function isCategoryId(v: unknown): v is number {
  return typeof v === 'number' && Number.isInteger(v) && v >= 0
}

// Reasonable bounds on the ranked-list row count — below 3 the widget barely
// shows anything useful, above 25 the bars get too thin to label.
const MIN_TOP_DESCRIPTIONS_LIMIT = 3
const MAX_TOP_DESCRIPTIONS_LIMIT = 25
const DEFAULT_TOP_DESCRIPTIONS_LIMIT = 10

function clampLimit(v: unknown): number {
  if (typeof v !== 'number' || !Number.isFinite(v)) return DEFAULT_TOP_DESCRIPTIONS_LIMIT
  return Math.min(MAX_TOP_DESCRIPTIONS_LIMIT, Math.max(MIN_TOP_DESCRIPTIONS_LIMIT, Math.round(v)))
}

function validEntries(v: unknown, isValid: (x: unknown) => boolean): number[] | null {
  if (!isNumberArray(v)) return null
  const kept = v.filter(isValid)
  return kept.length > 0 ? kept : null
}

// Grid spans are written straight into `gridColumn/gridRow: span N`, where a
// 0, a fraction or a negative is silently invalid CSS and a large number
// renders a card taller than the viewport.
function clampSpan(v: unknown, fallback: number): number {
  if (typeof v !== 'number' || !Number.isFinite(v)) return fallback
  return Math.min(4, Math.max(1, Math.round(v)))
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
      return { type: 'categoryChart', categoryIds: categoryIds.slice(0, MAX_CATEGORY_SLOTS), chartKind: normalizeChartKind(c.chartKind) }
    }
    case 'monthCompare': {
      const months = validEntries(c.months, isMonth)
      if (isYear(c.year) && months != null) {
        return { type: 'monthCompare', year: c.year, months, chartKind: normalizeChartKind(c.chartKind) }
      }
      return null
    }
    case 'allTimeTrend':
      if (isYear(c.fromYear) && isYear(c.toYear)) {
        return { type: 'allTimeTrend', fromYear: c.fromYear, toYear: c.toYear, chartKind: normalizeChartKind(c.chartKind) }
      }
      return null
    case 'monthAcrossYears': {
      const years = validEntries(c.years, isYear)
      if (isMonth(c.month) && years != null) {
        return { type: 'monthAcrossYears', month: c.month, years, chartKind: normalizeChartKind(c.chartKind) }
      }
      return null
    }
    case 'incomeSources': {
      const sourceIds = validEntries(c.sourceIds, isSourceId)
      if (sourceIds == null) return null
      return { type: 'incomeSources', sourceIds: sourceIds.slice(0, MAX_INCOME_SOURCE_SLOTS), chartKind: normalizeChartKind(c.chartKind) }
    }
    case 'incomeMix':
      return { type: 'incomeMix' }
    case 'itemizedIncome':
      if (isSourceId(c.sourceId)) {
        return { type: 'itemizedIncome', sourceId: c.sourceId }
      }
      return null
    case 'potBalances': {
      const potIds = validEntries(c.potIds, isPotId)
      if (potIds == null) return null
      return { type: 'potBalances', potIds: potIds.slice(0, MAX_POT_SLOTS), chartKind: normalizeChartKind(c.chartKind) }
    }
    case 'potFlow': {
      const potIds = validEntries(c.potIds, isPotId)
      if (potIds == null) return null
      return { type: 'potFlow', potIds: potIds.slice(0, MAX_POT_SLOTS) }
    }
    case 'potTargets':
      return { type: 'potTargets' }
    case 'savingsRate':
      if (isYear(c.year)) {
        return { type: 'savingsRate', year: c.year }
      }
      return null
    case 'categoryShare':
      return { type: 'categoryShare' }
    case 'topDescriptions': {
      const categoryId = c.categoryId === null ? null : (isCategoryId(c.categoryId) ? c.categoryId : undefined)
      if (categoryId === undefined) return null
      return { type: 'topDescriptions', categoryId, limit: clampLimit(c.limit) }
    }
    case 'sankeyFlow':
      if (isYear(c.year)) {
        return { type: 'sankeyFlow', year: c.year }
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
        width: clampSpan(w.width, 1) as WidgetWidth,
        height: clampSpan(w.height, defaultHeight(config)) as WidgetHeight,
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
