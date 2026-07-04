import { describe, it, expect } from 'vitest'
import { formatCents, parseToCents } from './money'

describe('formatCents', () => {
  it('formats NL locale', () => {
    const result = formatCents(172366, 'nl-NL')
    expect(result).toContain('1.723,66')
  })
  it('formats EN locale', () => {
    const result = formatCents(172366, 'en-US')
    expect(result).toContain('1,723.66')
  })
  it('handles zero', () => {
    expect(formatCents(0)).toContain('0,00')
  })
  it('handles negative', () => {
    expect(formatCents(-500)).toContain('-')
  })
})

describe('parseToCents', () => {
  it('parses NL format', () => {
    expect(parseToCents('1.723,66')).toBe(172366)
  })
  it('parses EN format', () => {
    expect(parseToCents('1,723.66')).toBe(172366)
  })
  it('parses plain decimal', () => {
    expect(parseToCents('17.50')).toBe(1750)
  })
  it('returns null for empty', () => {
    expect(parseToCents('')).toBeNull()
  })
  it('returns null for invalid', () => {
    expect(parseToCents('abc')).toBeNull()
  })
  it('handles negative', () => {
    expect(parseToCents('-10,50')).toBe(-1050)
  })
})
