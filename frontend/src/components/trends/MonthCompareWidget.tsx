import { Skeleton, Text } from '@mantine/core'
import { CompositeChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import { useYearSummary } from '../../api/hooks/usePeriods'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicks } from '../../lib/chartAxis'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'

const MONTHS_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

interface Props {
  year: number
  months: number[]
  chartKind: ChartKind
}

// Self-contained comparison of specific months within one year (e.g. "why
// was Jan pricier than Feb") — deliberately independent of the page's
// global year filter, per user request.
export default function MonthCompareWidget({ year, months, chartKind }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const monthNames = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const { palette } = useChartPalette()
  const { data, isLoading } = useYearSummary(year)

  if (isLoading) return <Skeleton h="100%" />
  if (!data) return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>

  const selected = [...months].sort((a, b) => a - b)
  const rows = selected.map((m) => data.months.find((row) => row.month === m)).filter((row) => row != null)

  if (rows.length === 0) {
    return <Text size="sm" c="dimmed">{t('trends.selectMonthsHint')}</Text>
  }

  const chartData = rows.map((row) => ({
    month: monthNames[row.month - 1],
    [t('year.income')]: row.incomeTotalCents / 100,
    [t('year.expenses')]: row.expenseTotalCents / 100,
    [t('year.surplus')]: row.surplusCents / 100,
  }))

  const markType = chartKind === 'bar' ? 'bar' : 'line'
  const series = [
    { name: t('year.income'), color: palette.income, type: markType === 'bar' ? ('bar' as const) : ('line' as const) },
    { name: t('year.expenses'), color: palette.expenses, type: markType === 'bar' ? ('bar' as const) : ('line' as const) },
    { name: t('year.surplus'), color: palette.surplus, type: 'line' as const },
  ]
  const maxValue = Math.max(...chartData.flatMap((row) => series.map((s) => Number(row[s.name]) || 0)), 0)
  const ticks = niceAxisTicks(maxValue)

  return (
    <>
      <div style={{ flexShrink: 0 }}>
        <ChartLegend series={series.map((s) => ({ name: s.name, color: s.color }))} />
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        <CompositeChart
          h="100%"
          data={chartData}
          dataKey="month"
          withLegend={false}
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          yAxisProps={{ tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale), width: 56, ticks, domain: [0, ticks[ticks.length - 1]] }}
          series={series}
        />
      </div>
    </>
  )
}
