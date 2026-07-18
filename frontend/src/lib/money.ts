export function formatCents(cents: number, locale = 'nl-NL'): string {
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'EUR',
  }).format(cents / 100)
}

// Short form for chart axis ticks (€100K instead of €100,000.00) — full
// precision there just forces charts to reserve more label width than the
// value is worth, which pushes wide labels past the edge of a narrow card.
export function formatCentsCompact(cents: number, locale = 'nl-NL'): string {
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'EUR',
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(cents / 100)
}

export function parseToCents(value: string): number | null {
  // Accept both "1.723,66" (NL) and "1,723.66" (EN) and plain "1723.66"
  if (!value || value.trim() === '') return null
  const cleaned = value.trim().replace(/[€\s]/g, '')
  // Detect format: if last separator is comma and it has 2 decimals → NL format
  const nlPattern = /^-?[\d.]*,\d{0,2}$/
  let normalized: string
  if (nlPattern.test(cleaned)) {
    normalized = cleaned.replace(/\./g, '').replace(',', '.')
  } else {
    normalized = cleaned.replace(/,/g, '')
  }
  const num = parseFloat(normalized)
  if (isNaN(num)) return null
  return Math.round(num * 100)
}
