import { memo } from 'react'
import { Skeleton, Text } from '@mantine/core'
import { LineChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import { useYearSummary } from '../../api/hooks/usePeriods'
import { niceAxisTicksSigned } from '../../lib/chartAxis'
import { monthNames } from '../../lib/monthNames'
import { useChartPalette } from '../../contexts/ChartPaletteContext'

interface Props {
  year: number
}

// Self-contained, own year like monthCompare/allTimeTrend — independent of
// the page's global year filter.
function SavingsRateWidget({ year }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const months = monthNames(i18n.language)
  const { palette } = useChartPalette()
  const { data, isLoading } = useYearSummary(year)

  if (isLoading) return <Skeleton h="100%" />
  if (!data) return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>

  const percentFormatter = new Intl.NumberFormat(locale, { style: 'percent', maximumFractionDigits: 0 })
  const seriesName = t('trends.savingsRateSeries')

  const chartData = months.map((label, i) => {
    const row = data.months.find((m) => m.month === i + 1)
    // A month with zero income has an undefined savings rate (division by
    // zero) — rendered as a gap in the line (null + connectNulls={false}),
    // never as a misleading 0%.
    const rate = row && row.incomeTotalCents > 0 ? row.surplusCents / row.incomeTotalCents : null
    return { month: label, [seriesName]: rate }
  })

  const values = chartData.map((row) => row[seriesName]).filter((v): v is number => v != null)
  const average = values.length > 0 ? values.reduce((sum, v) => sum + v, 0) / values.length : 0
  const ticks = niceAxisTicksSigned(Math.min(...values, 0), Math.max(...values, 0))

  return (
    <div style={{ flex: 1, minHeight: 0 }}>
      <LineChart
        h="100%"
        data={chartData}
        dataKey="month"
        withLegend={false}
        connectNulls={false}
        valueFormatter={(v: number) => percentFormatter.format(v)}
        series={[{ name: seriesName, color: palette.surplus }]}
        yAxisProps={{
          tickFormatter: (v: number) => percentFormatter.format(v),
          width: 56,
          ticks,
          domain: [ticks[0], ticks[ticks.length - 1]],
        }}
        // The average label sits top-RIGHT: January's marker is at the very
        // left of the plot and a left-aligned label lands on top of it.
        referenceLines={values.length > 0 ? [{ y: average, color: 'gray.5', label: percentFormatter.format(average), labelPosition: 'insideTopRight' }] : []}
      />
    </div>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(SavingsRateWidget)
