export type ChartKind = 'line' | 'bar'

export type WidgetConfig =
  | { type: 'kpi'; metric: 'income' | 'expenses' | 'surplus' | 'yearsTracked' }
  | { type: 'categoryChart'; categoryId: number; chartKind: ChartKind }
  | { type: 'yearCompare'; chartKind: ChartKind }
  // Self-contained: its own year + month selection, independent of the
  // page's global year filter — compares specific months within one year
  // (e.g. "why was Jan pricier than Feb") rather than years against
  // each other.
  | { type: 'monthCompare'; year: number; months: number[]; chartKind: ChartKind }

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
const DEFAULT_CATEGORY_COUNT = 8

function genId(): string {
  return Math.random().toString(36).slice(2, 10)
}

function defaultHeight(config: WidgetConfig): WidgetHeight {
  if (config.type === 'kpi') return 1
  if (config.type === 'yearCompare' || config.type === 'monthCompare') return 3
  return 2
}

export function defaultWidgets(categories: { id: number; name: string }[]): Widget[] {
  const configs: WidgetConfig[] = [
    { type: 'kpi', metric: 'income' },
    { type: 'kpi', metric: 'expenses' },
    { type: 'kpi', metric: 'surplus' },
    { type: 'kpi', metric: 'yearsTracked' },
    { type: 'yearCompare', chartKind: 'bar' },
    ...categories.slice(0, DEFAULT_CATEGORY_COUNT).map((cat): WidgetConfig => ({ type: 'categoryChart', categoryId: cat.id, chartKind: 'line' })),
  ]
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
    return (parsed.widgets as Widget[]).map((w) => ({
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
