import { Text } from '@mantine/core'
import { BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { YearTrend } from '../../api/types'
import MoneyText from '../MoneyText'
import { formatCents } from '../../lib/money'

const METRIC_COLOR: Record<string, string> = {
  income: 'teal.6',
  expenses: 'red.6',
  surplus: 'blue.6',
}

function metricCents(y: YearTrend, metric: 'income' | 'expenses' | 'surplus'): number {
  if (metric === 'income') return y.incomeTotalCents
  if (metric === 'expenses') return y.expenseTotalCents
  return y.surplusCents
}

interface Props {
  metric: 'income' | 'expenses' | 'surplus' | 'yearsTracked'
  filter: TrendsFilter
  allYears: YearTrend[]
}

export default function KpiWidget({ metric, filter, allYears }: Props) {
  const { i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  if (metric === 'yearsTracked') {
    return <Text size="lg" fw={700}>{allYears.length}</Text>
  }

  if (filter.mode === 'single') {
    const row = allYears.find((y) => y.year === filter.year)
    const cents = row ? metricCents(row, metric) : 0
    return <MoneyText cents={cents} size="lg" fw={700} colored={metric === 'surplus'} />
  }

  const years = [...filter.years].sort((a, b) => a - b)
  const data = years.map((y) => {
    const row = allYears.find((r) => r.year === y)
    return { year: String(y), value: (row ? metricCents(row, metric) : 0) / 100 }
  })

  return (
    <div style={{ flex: 1, minHeight: 0 }}>
      <BarChart
        h="100%"
        data={data}
        dataKey="year"
        withLegend={false}
        withYAxis={false}
        valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
        series={[{ name: 'value', color: METRIC_COLOR[metric] }]}
      />
    </div>
  )
}
