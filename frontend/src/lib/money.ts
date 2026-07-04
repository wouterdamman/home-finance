export function formatCents(cents: number, locale = 'nl-NL'): string {
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'EUR',
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
