import { memo } from 'react'
import { Skeleton, Text } from '@mantine/core'
import { CompositeChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import { useQueries } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { YearSummary } from '../../api/types'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicksSigned } from '../../lib/chartAxis'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'

const MONTHS_NL = ['jan', 'feb', 'mrt', 'apr', 'mei', 'jun', 'jul', 'aug', 'sep', 'okt', 'nov', 'dec']
const MONTHS_EN = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

interface Props {
  month: number
  years: number[]
  chartKind: ChartKind
}

// X-axis = years (one point per selected year), 3 fixed-color series for
// income/expenses/surplus of that single month — mirrors YearCompareWidget's
// convention rather than putting the 3 metrics on the x-axis, which would
// wrongly imply a continuum between unrelated measures.
function MonthAcrossYearsWidget({ month, years, chartKind }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const monthNames = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const { palette } = useChartPalette()
  const sortedYears = [...years].sort((a, b) => a - b)

  const results = useQueries({
    queries: sortedYears.map((year) => ({
      queryKey: ['year-summary', year],
      queryFn: () => api.get<YearSummary>(`/api/years/${year}/summary`),
    })),
  })

  if (sortedYears.length === 0) {
    return <Text size="sm" c="dimmed">{t('trends.selectYearsHint')}</Text>
  }
  if (results.some((r) => r.isLoading)) return <Skeleton h="100%" />
  if (results.some((r) => !r.data)) return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>

  const chartData = sortedYears.flatMap((year, i) => {
    const row = results[i].data!.months.find((mo) => mo.month === month)
    if (!row) return []
    return [{
      year: String(year),
      [t('year.income')]: row.incomeTotalCents / 100,
      [t('year.expenses')]: row.expenseTotalCents / 100,
      [t('year.surplus')]: row.surplusCents / 100,
    }]
  })

  if (chartData.length === 0) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  const markType = chartKind === 'bar' ? 'bar' : 'line'
  const series = [
    { name: t('year.income'), color: palette.income, type: markType === 'bar' ? ('bar' as const) : ('line' as const) },
    { name: t('year.expenses'), color: palette.expenses, type: markType === 'bar' ? ('bar' as const) : ('line' as const) },
    { name: t('year.surplus'), color: palette.surplus, type: 'line' as const },
  ]
  const values = chartData.flatMap((row) => series.map((s) => Number(row[s.name]) || 0))
  const ticks = niceAxisTicksSigned(Math.min(...values, 0), Math.max(...values, 0))

  return (
    <>
      <Text size="xs" c="dimmed" ta="center" style={{ flexShrink: 0 }}>{monthNames[month - 1]}</Text>
      <div style={{ flexShrink: 0 }}>
        <ChartLegend series={series.map((s) => ({ name: s.name, color: s.color }))} />
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        <CompositeChart
          h="100%"
          data={chartData}
          dataKey="year"
          withLegend={false}
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          yAxisProps={{ tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale), width: 56, ticks, domain: [ticks[0], ticks[ticks.length - 1]] }}
          series={series}
        />
      </div>
    </>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(MonthAcrossYearsWidget)
