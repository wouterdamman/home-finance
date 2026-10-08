import { memo } from 'react'
import { LineChart, BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { CategoryTotals } from '../../api/types'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicks, categoryTickInterval } from '../../lib/chartAxis'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import { categoryLabel } from '../../lib/categoryLabel'
import ChartLegend from './ChartLegend'
import { monthNames } from '../../lib/monthNames'

interface Props {
  categoryIds: number[]
  chartKind: ChartKind
  filter: TrendsFilter
  catData: CategoryTotals
}

function CategoryWidget({ categoryIds, chartKind, filter, catData }: Props) {
  const { t, i18n } = useTranslation()
  const months = monthNames(i18n.language)
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()
  const Chart = chartKind === 'bar' ? BarChart : LineChart

  const selectedCategories = categoryIds
    .map((id) => catData.categories.find((c) => c.id === id))
    .filter((c): c is NonNullable<typeof c> => c != null)
    .map((cat) => ({ id: cat.id, name: categoryLabel(cat, t) }))

  const data = months.map((label, i) => {
    const entry = catData.entries.find((e) => e.year === filter.year && e.month === i + 1)
    const row: Record<string, string | number> = { month: label }
    for (const cat of selectedCategories) {
      row[cat.name] = (entry?.values[String(cat.id)] ?? 0) / 100
    }
    return row
  })

  const series = selectedCategories.map((cat, idx) => ({ name: cat.name, color: palette.categorical[idx] }))
  const maxValue = Math.max(...data.flatMap((row) => series.map((s) => Number(row[s.name]) || 0)), 0)
  const ticks = niceAxisTicks(maxValue)
  const xInterval = categoryTickInterval(data.length)

  return (
    <>
      {series.length > 1 && (
        <div style={{ flexShrink: 0 }}>
          <ChartLegend series={series} />
        </div>
      )}
      <div style={{ flex: 1, minHeight: 0 }}>
        <Chart
          h="100%"
          data={data}
          dataKey="month"
          withLegend={false}
          valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
          series={series}
          xAxisProps={{ interval: xInterval }}
          yAxisProps={{ tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale), width: 56, ticks, domain: [0, ticks[ticks.length - 1]] }}
        />
      </div>
    </>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(CategoryWidget)
