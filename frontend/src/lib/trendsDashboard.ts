export type ChartKind = 'line' | 'bar'

export type WidgetConfig =
  | { type: 'kpi'; metric: 'income' | 'expenses' | 'surplus' | 'yearsTracked' }
  | { type: 'categoryChart'; categoryId: number; chartKind: ChartKind }
  | { type: 'yearCompare'; chartKind: ChartKind }

export interface Widget {
  id: string
  visible: boolean
  config: WidgetConfig
}

const STORAGE_KEY = 'trends-dashboard-v1'
const DEFAULT_CATEGORY_COUNT = 8

function genId(): string {
  return Math.random().toString(36).slice(2, 10)
}

export function defaultWidgets(categories: { id: number; name: string }[]): Widget[] {
  const kpiMetrics: WidgetConfig[] = [
    { type: 'kpi', metric: 'income' },
    { type: 'kpi', metric: 'expenses' },
    { type: 'kpi', metric: 'surplus' },
    { type: 'kpi', metric: 'yearsTracked' },
  ]
  const widgets: Widget[] = kpiMetrics.map((config) => ({ id: genId(), visible: true, config }))
  widgets.push({ id: genId(), visible: true, config: { type: 'yearCompare', chartKind: 'bar' } })
  for (const cat of categories.slice(0, DEFAULT_CATEGORY_COUNT)) {
    widgets.push({ id: genId(), visible: true, config: { type: 'categoryChart', categoryId: cat.id, chartKind: 'line' } })
  }
  return widgets
}

export function loadDashboard(): Widget[] | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as { widgets?: unknown }
    if (!Array.isArray(parsed.widgets)) return null
    return parsed.widgets as Widget[]
  } catch {
    return null
  }
}

export function saveDashboard(widgets: Widget[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ widgets }))
}

export function newWidget(config: WidgetConfig): Widget {
  return { id: genId(), visible: true, config }
}
