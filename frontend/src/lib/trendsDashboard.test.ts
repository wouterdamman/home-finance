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
})
