import { memo } from 'react'
import { AreaChart, BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { TrendsIncomeSources } from '../../api/types'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicks, categoryTickInterval } from '../../lib/chartAxis'
import { monthNames } from '../../lib/monthNames'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'

interface Props {
  sourceIds: number[]
  chartKind: ChartKind
  filter: TrendsFilter
  incomeData: TrendsIncomeSources
}

// 'bar' renders a stacked BarChart; 'line' (the same SegmentedControl used
// by every other widget's chartKind picker) renders a stacked AreaChart
// instead of a plain LineChart — a set of per-source lines wouldn't stack,
// and stacking is the whole point of this widget.
function IncomeSourcesWidget({ sourceIds, chartKind, filter, incomeData }: Props) {
  const { i18n } = useTranslation()
  const months = monthNames(i18n.language)
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  const selectedSources = sourceIds
    .map((id) => incomeData.sources.find((s) => s.id === id))
    .filter((s): s is NonNullable<typeof s> => s != null)

  const data = months.map((label, i) => {
    const entry = incomeData.entries.find((e) => e.year === filter.year && e.month === i + 1)
    const row: Record<string, string | number> = { month: label }
    for (const source of selectedSources) {
      row[source.name] = (entry?.values[String(source.id)] ?? 0) / 100
    }
    return row
  })

  const series = selectedSources.map((source, idx) => ({ name: source.name, color: palette.categorical[idx] }))
  const maxValue = Math.max(...data.flatMap((row) => series.map((s) => Number(row[s.name]) || 0)), 0)
  const ticks = niceAxisTicks(maxValue)
  const xInterval = categoryTickInterval(data.length)

  // A visible gap between stacked segments, per the dataviz standard —
  // stroked in the surface color so adjacent segments never visually merge.
  const segmentGap = { strokeWidth: 2, stroke: 'var(--mantine-color-body)' }

  const sharedProps = {
    h: '100%' as const,
    data,
    dataKey: 'month',
    type: 'stacked' as const,
    withLegend: false,
    valueFormatter: (v: number) => formatCents(Math.round(v * 100), locale),
    series,
    xAxisProps: { interval: xInterval },
    yAxisProps: {
      tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale),
      width: 56,
      ticks,
      domain: [0, ticks[ticks.length - 1]] as [number, number],
    },
  }

  return (
    <>
      {series.length > 1 && (
        <div style={{ flexShrink: 0 }}>
          <ChartLegend series={series} />
        </div>
      )}
      <div style={{ flex: 1, minHeight: 0 }}>
        {chartKind === 'bar' ? (
          <BarChart {...sharedProps} barProps={() => segmentGap} />
        ) : (
          <AreaChart {...sharedProps} areaProps={() => segmentGap} />
        )}
      </div>
    </>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(IncomeSourcesWidget)
