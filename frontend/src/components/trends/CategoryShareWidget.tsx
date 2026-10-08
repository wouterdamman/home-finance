import { memo } from 'react'
import { Text, Center } from '@mantine/core'
import { DonutChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { CategoryTotals, TrendsMonthlyTotal } from '../../api/types'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'

interface Props {
  filter: TrendsFilter
  catData: CategoryTotals
  monthlyTotals: TrendsMonthlyTotal[]
}

// palette.categorical has 8 slots; the top 7 real categories get slots 0-6,
// slot 7 is reserved for the "Other categories" fold (smaller categories
// beyond the top 7) — same convention as IncomeMixWidget.
const MAX_REAL_SLICES = 7
// The "no category" gap is a different kind of bucket than "Other
// categories" (it's budget lines with no category at all, not a fold of
// small real ones) — rendered in a neutral gray rather than a categorical
// slot so it never reads as an 8th/9th entity competing for the same colors.
const UNCATEGORIZED_COLOR = 'gray.6'

function CategoryShareWidget({ filter, catData, monthlyTotals }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  // /api/trends/category-totals omits budget lines that have no category,
  // so summing the categories alone under-reports the year's real expense
  // total — the year's expense total from monthly-totals is the source of
  // truth, and the gap between the two becomes its own explicit slice
  // instead of silently vanishing.
  const yearExpenseTotalCents = monthlyTotals
    .filter((m) => m.year === filter.year)
    .reduce((sum, m) => sum + m.expenseTotalCents, 0)

  if (yearExpenseTotalCents <= 0) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  const categoryTotals = catData.categories
    .map((cat) => ({
      name: cat.name,
      totalCents: catData.entries
        .filter((e) => e.year === filter.year)
        .reduce((sum, e) => sum + (e.values[String(cat.id)] ?? 0), 0),
    }))
    .sort((a, b) => b.totalCents - a.totalCents)

  // A donut cannot draw a negative slice, so categories that net out at or
  // below zero for the year (refunds, correction lines) are not shown — but
  // their value still has to come off the "uncategorised" gap below, which
  // is derived from the year total they are part of. Dropping them before
  // computing the gap understated it and could clamp it to 0, hiding real
  // uncategorised spend.
  const categorizedCents = categoryTotals.reduce((sum, c) => sum + c.totalCents, 0)
  const positiveTotals = categoryTotals.filter((c) => c.totalCents > 0)

  const top = positiveTotals.slice(0, MAX_REAL_SLICES)
  const otherCents = positiveTotals.slice(MAX_REAL_SLICES).reduce((sum, c) => sum + c.totalCents, 0)
  // Rounding/timing differences between the two endpoints could in theory
  // push the computed gap slightly negative — clamp rather than render a
  // slice with a negative value.
  const uncategorizedCents = Math.max(0, yearExpenseTotalCents - categorizedCents)

  const chartData = top.map((c, idx) => ({ name: c.name, value: c.totalCents / 100, color: palette.categorical[idx] }))
  if (otherCents > 0) {
    chartData.push({ name: t('trends.otherCategories'), value: otherCents / 100, color: palette.categorical[7] })
  }
  if (uncategorizedCents > 0) {
    chartData.push({ name: t('trends.uncategorizedCategory'), value: uncategorizedCents / 100, color: UNCATEGORIZED_COLOR })
  }

  const legendSeries = chartData.map((d) => ({ name: d.name, color: d.color }))

  return (
    <>
      {legendSeries.length > 1 && (
        <div style={{ flexShrink: 0 }}>
          <ChartLegend series={legendSeries} />
        </div>
      )}
      <Center style={{ flex: 1, minHeight: 0 }}>
        <DonutChart
          data={chartData}
          withTooltip
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          chartLabel={formatCentsCompact(yearExpenseTotalCents, locale)}
          size={150}
          thickness={28}
          paddingAngle={2}
          strokeWidth={2}
        />
      </Center>
    </>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(CategoryShareWidget)
