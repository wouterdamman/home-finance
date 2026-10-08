// Shared short month-name arrays used across the /trends widgets and pages.
export const MONTH_NAMES_NL = ['jan', 'feb', 'mrt', 'apr', 'mei', 'jun', 'jul', 'aug', 'sep', 'okt', 'nov', 'dec']
export const MONTH_NAMES_EN = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

// Same `language.startsWith('nl')` rule used throughout (see Trends.tsx).
export function monthNames(language: string): string[] {
  return language.startsWith('nl') ? MONTH_NAMES_NL : MONTH_NAMES_EN
}
