import { LineChart, BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import { YEAR_COLORS } from '../../lib/trendsFilter'
import type { CategoryTotals } from '../../api/types'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents } from '../../lib/money'
import { niceAxisTicks } from '../../lib/chartAxis'
import ChartLegend from './ChartLegend'

const MONTHS_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

interface Props {
  categoryId: number
  chartKind: ChartKind
  filter: TrendsFilter
  catData: CategoryTotals
}

export default function CategoryWidget({ categoryId, chartKind, filter, catData }: Props) {
  const { i18n } = useTranslation()
  const months = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const Chart = chartKind === 'bar' ? BarChart : LineChart

  if (filter.mode === 'single') {
    const data = months.map((label, i) => {
      const entry = catData.entries.find((e) => e.year === filter.year && e.month === i + 1)
      return { month: label, value: (entry?.values[String(categoryId)] ?? 0) / 100 }
    })
    const ticks = niceAxisTicks(Math.max(...data.map((d) => d.value), 0))
    return (
      <div style={{ flex: 1, minHeight: 0 }}>
        <Chart
          h="100%"
          data={data}
          dataKey="month"
          withLegend={false}
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          series={[{ name: 'value', color: 'teal.6' }]}
          yAxisProps={{ ticks, domain: [0, ticks[ticks.length - 1]] }}
        />
      </div>
    )
  }

  const years = [...filter.years].sort((a, b) => a - b)
  const data = months.map((label, i) => {
    const row: Record<string, string | number> = { month: label }
    for (const y of years) {
      const entry = catData.entries.find((e) => e.year === y && e.month === i + 1)
      row[String(y)] = (entry?.values[String(categoryId)] ?? 0) / 100
    }
    return row
  })
  const series = years.map((y, idx) => ({ name: String(y), color: YEAR_COLORS[idx] }))
  const maxValue = Math.max(...data.flatMap((row) => years.map((y) => Number(row[String(y)]) || 0)), 0)
  const ticks = niceAxisTicks(maxValue)

  return (
    <>
      <div style={{ flexShrink: 0 }}><ChartLegend series={series} /></div>
      <div style={{ flex: 1, minHeight: 0 }}>
        <Chart
          h="100%"
          data={data}
          dataKey="month"
          withLegend={false}
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          series={series}
          yAxisProps={{ ticks, domain: [0, ticks[ticks.length - 1]] }}
        />
      </div>
    </>
  )
}
