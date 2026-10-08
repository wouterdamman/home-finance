import { memo } from 'react'
import { Text, Center } from '@mantine/core'
import { DonutChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { TrendsIncomeSources } from '../../api/types'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'
import { incomeSourceLabel } from '../../lib/incomeSourceLabel'

interface Props {
  filter: TrendsFilter
  incomeData: TrendsIncomeSources
}

// Shares below this threshold fold into one explicit "Other" slice instead
// of cluttering the donut with slivers — same idea as the categorical
// palette's "a 9th series is never a generated hue" rule, just triggered by
// share size instead of raw count.
const SMALL_SHARE_THRESHOLD = 0.03
// palette.categorical has 8 slots; the last one is reserved for "Other" so
// a 9th real entity never needs a generated color.
const MAX_REAL_SLICES = 7

function IncomeMixWidget({ filter, incomeData }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  const totals = incomeData.sources
    .map((source) => ({
      name: incomeSourceLabel(source, t),
      totalCents: incomeData.entries
        .filter((e) => e.year === filter.year)
        .reduce((sum, e) => sum + (e.values[String(source.id)] ?? 0), 0),
    }))
    .filter((s) => s.totalCents > 0)

  const grandTotal = totals.reduce((sum, s) => sum + s.totalCents, 0)
  if (grandTotal <= 0) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  const sorted = [...totals].sort((a, b) => b.totalCents - a.totalCents)
  const big = sorted.filter((s) => s.totalCents / grandTotal >= SMALL_SHARE_THRESHOLD)
  const kept = big.slice(0, MAX_REAL_SLICES)
  const otherCents = grandTotal - kept.reduce((sum, s) => sum + s.totalCents, 0)

  const chartData = kept.map((s, idx) => ({ name: s.name, value: s.totalCents / 100, color: palette.categorical[idx] }))
  if (otherCents > 0) {
    chartData.push({ name: t('trends.otherSource'), value: otherCents / 100, color: palette.categorical[7] })
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
          chartLabel={formatCentsCompact(grandTotal, locale)}
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
export default memo(IncomeMixWidget)
