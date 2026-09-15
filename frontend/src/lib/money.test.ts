import { describe, it, expect } from 'vitest'
import { formatCents, formatCentsCompact, parseToCents } from './money'

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

describe('formatCentsCompact', () => {
  it('leaves sub-thousand amounts unabbreviated', () => {
    expect(formatCentsCompact(99900, 'nl-NL')).toContain('999')
  })
  it('abbreviates thousands', () => {
    expect(formatCentsCompact(150000, 'en-US')).toContain('1.5K')
  })
  it('abbreviates thousands in NL locale with a comma decimal', () => {
    expect(formatCentsCompact(150000, 'nl-NL')).toContain('1,5K')
  })
  it('keeps at most one fraction digit', () => {
    expect(formatCentsCompact(1234567, 'en-US')).toContain('12.3K')
  })
  it('abbreviates millions', () => {
    expect(formatCentsCompact(100000000, 'en-US')).toContain('1M')
  })
  it('handles zero', () => {
    expect(formatCentsCompact(0, 'en-US')).toContain('0')
  })
  it('keeps the sign on negatives', () => {
    expect(formatCentsCompact(-250000, 'en-US')).toContain('-')
  })
  it('defaults to the NL locale', () => {
    expect(formatCentsCompact(150000)).toContain('1,5K')
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
  it('reads a dot-grouped amount with no decimals as NL thousands', () => {
    expect(parseToCents('1.234')).toBe(123400)
  })
  it('reads multiple dot-separated groups as NL thousands', () => {
    expect(parseToCents('1.234.567')).toBe(123456700)
  })
  it('still reads a 2-digit group after a dot as EN decimals', () => {
    expect(parseToCents('17.5')).toBe(1750)
  })
  it('returns null for whitespace only', () => {
    expect(parseToCents('   ')).toBeNull()
  })
})
