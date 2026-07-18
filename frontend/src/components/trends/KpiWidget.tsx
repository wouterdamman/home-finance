import { Text } from '@mantine/core'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { YearTrend } from '../../api/types'
import MoneyText from '../MoneyText'

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
  if (metric === 'yearsTracked') {
    return <Text size="lg" fw={700}>{allYears.length}</Text>
  }
  const row = allYears.find((y) => y.year === filter.year)
  const cents = row ? metricCents(row, metric) : 0
  return <MoneyText cents={cents} size="lg" fw={700} colored={metric === 'surplus'} />
}
