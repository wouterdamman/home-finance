import { describe, it, expect } from 'vitest'
import { niceAxisTicks, niceAxisTicksSigned } from './chartAxis'

describe('niceAxisTicks', () => {
  it('returns [0] for a non-positive max', () => {
    expect(niceAxisTicks(0)).toEqual([0])
    expect(niceAxisTicks(-100)).toEqual([0])
  })

  it('produces evenly-spaced ticks starting at 0', () => {
    const ticks = niceAxisTicks(1800)
    expect(ticks[0]).toBe(0)
    expect(ticks[ticks.length - 1]).toBeGreaterThanOrEqual(1800)
    for (let i = 1; i < ticks.length; i++) {
      expect(ticks[i] - ticks[i - 1]).toBeCloseTo(ticks[1] - ticks[0])
    }
  })
})

describe('niceAxisTicksSigned', () => {
  it('matches niceAxisTicks when minValue is 0', () => {
    expect(niceAxisTicksSigned(0, 1800)).toEqual(niceAxisTicks(1800))
  })

  it('returns [0] when both min and max collapse to zero', () => {
    expect(niceAxisTicksSigned(0, 0)).toEqual([0])
    expect(niceAxisTicksSigned(5, -5)).toEqual([0])
  })

  it('extends the axis below zero for a negative surplus month', () => {
    // e.g. income 1800, expenses 2100 -> surplus -300
    const ticks = niceAxisTicksSigned(-300, 1800)
    expect(ticks[0]).toBeLessThan(0)
    expect(ticks[0]).toBeLessThanOrEqual(-300)
    expect(ticks[ticks.length - 1]).toBeGreaterThanOrEqual(1800)
    expect(ticks).toContain(0)
  })

  it('handles an all-negative range', () => {
    const ticks = niceAxisTicksSigned(-900, -100)
    expect(ticks[0]).toBeLessThanOrEqual(-900)
    expect(ticks[ticks.length - 1]).toBeGreaterThanOrEqual(0)
  })

  it('ignores a positive minValue (never clips the axis above 0)', () => {
    expect(niceAxisTicksSigned(50, 1800)).toEqual(niceAxisTicks(1800))
  })
})
