import { describe, it, expect } from 'vitest'
import { monthNames, MONTH_NAMES_NL, MONTH_NAMES_EN } from './monthNames'

describe('monthNames', () => {
  it('returns the Dutch names for a language starting with nl', () => {
    expect(monthNames('nl')).toBe(MONTH_NAMES_NL)
    expect(monthNames('nl-NL')).toBe(MONTH_NAMES_NL)
  })

  it('returns the English names for any other language', () => {
    expect(monthNames('en')).toBe(MONTH_NAMES_EN)
    expect(monthNames('en-US')).toBe(MONTH_NAMES_EN)
    expect(monthNames('fr')).toBe(MONTH_NAMES_EN)
  })

  it('has exactly 12 entries in each list', () => {
    expect(MONTH_NAMES_NL).toHaveLength(12)
    expect(MONTH_NAMES_EN).toHaveLength(12)
  })
})
