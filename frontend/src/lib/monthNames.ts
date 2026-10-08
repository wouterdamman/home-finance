// Shared short month-name arrays for new /trends widgets. Several existing
// widgets/pages already duplicate this exact data inline (e.g. Trends.tsx) —
// that duplication is deliberately left alone; this helper is only for code
// written from here on.
export const MONTH_NAMES_NL = ['jan', 'feb', 'mrt', 'apr', 'mei', 'jun', 'jul', 'aug', 'sep', 'okt', 'nov', 'dec']
export const MONTH_NAMES_EN = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

// Same `language.startsWith('nl')` rule used throughout (see Trends.tsx).
export function monthNames(language: string): string[] {
  return language.startsWith('nl') ? MONTH_NAMES_NL : MONTH_NAMES_EN
}
