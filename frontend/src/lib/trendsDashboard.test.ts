import { describe, it, expect, beforeEach } from 'vitest'
import { loadDashboard, saveDashboard, type Widget } from './trendsDashboard'

const STORAGE_KEY = 'trends-dashboard-v1'

describe('loadDashboard', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns null when nothing is stored', () => {
    expect(loadDashboard()).toBeNull()
  })

  it('migrates a pre-multi-select categoryChart widget (categoryId -> categoryIds)', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        widgets: [
          { id: 'a', visible: true, width: 1, height: 2, config: { type: 'categoryChart', categoryId: 42, chartKind: 'line' } },
        ],
      }),
    )
    const widgets = loadDashboard()
    expect(widgets).toHaveLength(1)
    expect(widgets![0].config).toEqual({ type: 'categoryChart', categoryIds: [42], chartKind: 'line' })
  })

  it('drops a categoryChart widget with neither categoryIds nor categoryId', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'categoryChart', chartKind: 'line' } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('drops widgets of a retired type instead of crashing', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'yearCompare' } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('defaults width/height when missing from an older save', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'kpi', metric: 'income' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].width).toBe(1)
    expect(widgets![0].height).toBe(1)
  })

  it('drops a monthAcrossYears widget whose stored month is out of range', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'monthAcrossYears', month: 13, years: [2025] } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('drops a monthAcrossYears widget whose stored month is zero', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'monthAcrossYears', month: 0, years: [2025] } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('filters out-of-range months from a monthCompare widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'monthCompare', year: 2025, months: [0, 3, 99, 7] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'monthCompare', year: 2025, months: [3, 7], chartKind: 'line' })
  })

  it('drops a monthCompare widget left with no valid months', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'monthCompare', year: 2025, months: [0, 13] } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('caps categoryIds at the four palette slots', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'categoryChart', categoryIds: [1, 2, 3, 4, 5, 6] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'categoryChart', categoryIds: [1, 2, 3, 4], chartKind: 'line' })
  })

  it('clamps out-of-range widget spans instead of writing invalid grid CSS', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        widgets: [
          { id: 'a', visible: true, width: 0, height: 40, config: { type: 'kpi', metric: 'income' } },
          { id: 'b', visible: true, width: -2, height: 2.5, config: { type: 'kpi', metric: 'expenses' } },
        ],
      }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].width).toBe(1)
    expect(widgets![0].height).toBe(4)
    expect(widgets![1].width).toBe(1)
    expect(widgets![1].height).toBe(3)
  })

  it('round-trips a fresh multi-category widget unchanged', () => {
    const widgets: Widget[] = [
      { id: 'a', visible: true, width: 2, height: 2, config: { type: 'categoryChart', categoryIds: [1, 2, 3], chartKind: 'bar' } },
    ]
    saveDashboard(widgets)
    expect(loadDashboard()).toEqual(widgets)
  })

  it('caps incomeSources sourceIds at the eight palette slots', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeSources', sourceIds: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'incomeSources', sourceIds: [0, 1, 2, 3, 4, 5, 6, 7], chartKind: 'line' })
  })

  it('allows source id 0 (the carryover bucket) in an incomeSources widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeSources', sourceIds: [0], chartKind: 'bar' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'incomeSources', sourceIds: [0], chartKind: 'bar' })
  })

  it('drops an incomeSources widget with no valid sourceIds', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeSources', sourceIds: [-1, 'nope'] } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('drops an incomeSources widget with a missing sourceIds field', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeSources', chartKind: 'line' } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('defaults a bare incomeMix widget (no fields beyond type)', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeMix' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'incomeMix' })
    expect(widgets![0].height).toBe(3)
  })

  it('ignores garbage fields persisted on an incomeMix widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeMix', sourceIds: [1, 2], chartKind: 'bar' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'incomeMix' })
  })

  it('keeps a valid itemizedIncome widget, including source id 0', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'itemizedIncome', sourceId: 0 } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'itemizedIncome', sourceId: 0 })
  })

  it('drops an itemizedIncome widget whose sourceId is missing or not a number', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        widgets: [
          { id: 'a', visible: true, config: { type: 'itemizedIncome' } },
          { id: 'b', visible: true, config: { type: 'itemizedIncome', sourceId: 'three' } },
          { id: 'c', visible: true, config: { type: 'itemizedIncome', sourceId: -1 } },
        ],
      }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('defaults incomeSources to a 3-row height when missing from an older save', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'incomeSources', sourceIds: [1] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].height).toBe(3)
  })

  it('caps potBalances potIds at the eight palette slots', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potBalances', potIds: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'potBalances', potIds: [0, 1, 2, 3, 4, 5, 6, 7], chartKind: 'line' })
  })

  it('drops a potBalances widget with no valid potIds', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potBalances', potIds: [-1, 'nope'] } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('drops a potBalances widget with a missing potIds field', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potBalances', chartKind: 'bar' } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('defaults potBalances to a 3-row height when missing from an older save', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potBalances', potIds: [1] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].height).toBe(3)
  })

  it('keeps a valid potFlow widget, including pot id 0', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potFlow', potIds: [0, 2] } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'potFlow', potIds: [0, 2] })
  })

  it('ignores a chartKind persisted on a potFlow widget — it has none', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potFlow', potIds: [1], chartKind: 'bar' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'potFlow', potIds: [1] })
  })

  it('drops a potFlow widget with no valid potIds', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potFlow', potIds: [] } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('defaults a bare potTargets widget (no fields beyond type)', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potTargets' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'potTargets' })
    expect(widgets![0].height).toBe(3)
  })

  it('ignores garbage fields persisted on a potTargets widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'potTargets', potIds: [1, 2], chartKind: 'bar' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'potTargets' })
  })

  it('keeps a valid savingsRate widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'savingsRate', year: 2025 } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'savingsRate', year: 2025 })
    expect(widgets![0].height).toBe(3)
  })

  it('drops a savingsRate widget whose year is missing or out of range', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        widgets: [
          { id: 'a', visible: true, config: { type: 'savingsRate' } },
          { id: 'b', visible: true, config: { type: 'savingsRate', year: 'nope' } },
        ],
      }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('defaults a bare categoryShare widget (no fields beyond type)', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'categoryShare' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'categoryShare' })
    expect(widgets![0].height).toBe(3)
  })

  it('ignores garbage fields persisted on a categoryShare widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'categoryShare', categoryIds: [1, 2], chartKind: 'bar' } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'categoryShare' })
  })

  it('keeps a topDescriptions widget with categoryId: null (all categories)', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'topDescriptions', categoryId: null, limit: 10 } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'topDescriptions', categoryId: null, limit: 10 })
    expect(widgets![0].height).toBe(3)
  })

  it('keeps a topDescriptions widget scoped to one category', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'topDescriptions', categoryId: 7, limit: 5 } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'topDescriptions', categoryId: 7, limit: 5 })
  })

  it('clamps an out-of-range topDescriptions limit instead of dropping the widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'topDescriptions', categoryId: null, limit: 999 } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'topDescriptions', categoryId: null, limit: 25 })
  })

  it('defaults a missing topDescriptions limit instead of dropping the widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'topDescriptions', categoryId: null } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'topDescriptions', categoryId: null, limit: 10 })
  })

  it('drops a topDescriptions widget whose categoryId is neither null nor a valid id', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'topDescriptions', categoryId: 'nope' } }] }),
    )
    expect(loadDashboard()).toEqual([])
  })

  it('keeps a valid sankeyFlow widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ widgets: [{ id: 'a', visible: true, config: { type: 'sankeyFlow', year: 2025 } }] }),
    )
    const widgets = loadDashboard()
    expect(widgets![0].config).toEqual({ type: 'sankeyFlow', year: 2025 })
    expect(widgets![0].height).toBe(4)
  })

  it('drops a sankeyFlow widget whose year is missing or out of range', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        widgets: [
          { id: 'a', visible: true, config: { type: 'sankeyFlow' } },
          { id: 'b', visible: true, config: { type: 'sankeyFlow', year: 'nope' } },
        ],
      }),
    )
    expect(loadDashboard()).toEqual([])
  })
})
