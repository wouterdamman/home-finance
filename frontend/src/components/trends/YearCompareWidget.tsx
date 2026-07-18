import { Text } from '@mantine/core'
import { CompositeChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { YearTrend } from '../../api/types'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents, formatCentsCompact } from '../../lib/money'
import ChartLegend from './ChartLegend'

interface Props {
  chartKind: ChartKind
  allYears: YearTrend[]
  onYearClick: (year: number) => void
}

interface ChartClickEvent {
  activeLabel?: string | number
}

export default function YearCompareWidget({ chartKind, allYears, onYearClick }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  const data = allYears.map((y) => ({
    year: String(y.year),
    [t('year.income')]: y.incomeTotalCents / 100,
    [t('year.expenses')]: y.expenseTotalCents / 100,
    [t('year.surplus')]: y.surplusCents / 100,
  }))

  const handleClick = (e: ChartClickEvent | undefined) => {
    if (e?.activeLabel) onYearClick(Number(e.activeLabel))
  }

  // "bar" kind keeps income/expenses as bars with surplus as a line (the
  // established convention from YearDashboard); "line" renders all three as
  // lines — same three series, just a different mark per series.
  const markType = chartKind === 'bar' ? 'bar' : 'line'
  const series = [
    { name: t('year.income'), color: 'teal.6', type: markType === 'bar' ? ('bar' as const) : ('line' as const) },
    { name: t('year.expenses'), color: 'red.6', type: markType === 'bar' ? ('bar' as const) : ('line' as const) },
    { name: t('year.surplus'), color: 'blue.6', type: 'line' as const },
  ]

  return (
    <div>
      <ChartLegend series={series.map((s) => ({ name: s.name, color: s.color }))} />
      <div style={{ cursor: 'pointer' }}>
        <CompositeChart
          h={220}
          data={data}
          dataKey="year"
          withLegend={false}
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          yAxisProps={{ tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale), width: 56 }}
          series={series}
          composedChartProps={{ onClick: handleClick }}
        />
      </div>
      <Text size="xs" c="dimmed" ta="center" mt={2}>{t('trends.clickYearHint')}</Text>
    </div>
  )
}
