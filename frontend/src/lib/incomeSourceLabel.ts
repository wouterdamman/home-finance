import type { TrendsIncomeSourcesSource } from '../api/types'

// /api/trends/income-sources buckets carryover income entries (which have no
// source of their own) under a synthetic source with id 0. The backend sends
// an English placeholder name for it; the UI is translated, so resolve the
// label here instead of rendering whatever the API happened to say.
export const CARRYOVER_SOURCE_ID = 0

export function incomeSourceLabel(
  source: Pick<TrendsIncomeSourcesSource, 'id' | 'name'>,
  t: (key: string) => string,
): string {
  return source.id === CARRYOVER_SOURCE_ID ? t('trends.carryoverSource') : source.name
}
