import type { CategoryTotalsCategory } from '../api/types'

// /api/trends/category-totals buckets label-only budget lines (category_id
// NULL — a line with no category, just a label) under a synthetic category
// with id 0, same convention as incomeSourceLabel's carryover bucket. The
// backend sends an English placeholder name for it; the UI is translated,
// so resolve the label here instead of rendering whatever the API happened
// to say.
export const UNCATEGORIZED_CATEGORY_ID = 0

export function categoryLabel(
  category: Pick<CategoryTotalsCategory, 'id' | 'name'>,
  t: (key: string) => string,
): string {
  return category.id === UNCATEGORIZED_CATEGORY_ID ? t('trends.uncategorizedCategory') : category.name
}
