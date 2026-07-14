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
  it('parses a whole euro in NL format', () => {
    expect(parseToCents('1,00')).toBe(100)
  })
  it('parses cents-only NL format', () => {
    expect(parseToCents('0,05')).toBe(5)
  })
  it('parses a plain integer with no separator', () => {
    expect(parseToCents('1234')).toBe(123400)
  })
  it('strips euro sign and surrounding whitespace', () => {
    expect(parseToCents(' € 12,50 ')).toBe(1250)
  })
  it('parses large NL-formatted thousands', () => {
    expect(parseToCents('12.345,67')).toBe(1234567)
  })
  it('treats a 3-digit group after a comma as EN thousands (ambiguous input)', () => {
    // "1,234" doesn't match the NL decimal pattern (max 2 digits after comma),
    // so it falls through to the EN thousands-separator branch: 1234 units.
    expect(parseToCents('1,234')).toBe(123400)
  })
  it('returns null for whitespace only', () => {
    expect(parseToCents('   ')).toBeNull()
  })
})
