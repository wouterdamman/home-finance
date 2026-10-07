import { memo } from 'react'
import { BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { TrendsPotBalances } from '../../api/types'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicksSigned, categoryTickInterval } from '../../lib/chartAxis'
import { monthNames } from '../../lib/monthNames'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'

interface Props {
  potIds: number[]
  filter: TrendsFilter
  potData: TrendsPotBalances
}

// Always bars, never line/area: a signed stacked area with both a positive
// and a negative stack reads far worse than two separate bar stacks on the
// same zero baseline, so (unlike potBalances) this type has no chartKind.
function PotFlowWidget({ potIds, filter, potData }: Props) {
  const { i18n } = useTranslation()
  const months = monthNames(i18n.language)
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  const selectedPots = potIds
    .map((id) => potData.pots.find((p) => p.id === id))
    .filter((p): p is NonNullable<typeof p> => p != null)

  const data = months.map((label, i) => {
    const entry = potData.entries.find((e) => e.year === filter.year && e.month === i + 1)
    const row: Record<string, string | number> = { month: label }
    for (const pot of selectedPots) {
      const key = String(pot.id)
      row[`${key}__in`] = (entry?.inflow[key] ?? 0) / 100
      // pot_ledger amounts are signed, so the API's outflow sum arrives
      // negative (withdrawals) — but an `adjustment` entry keeps its own sign
      // and can be positive. Normalising to -|x| is what guarantees outflow
      // always renders below the zero line, on the same signed y-axis as
      // inflow above it. NEVER a second y-axis for this.
      row[`${key}__out`] = -Math.abs((entry?.outflow[key] ?? 0) / 100)
    }
    return row
  })

  // Two series per pot (same color — identity is the pot, not the
  // direction), each on its own stackId so inflow stacks upward and outflow
  // stacks downward independently.
  // Keyed by pot id rather than name: pot names are not unique (an archived
  // pot and a recreated namesake both appear here) and a name key would make
  // one pot's bars overwrite the other's.
  const series = selectedPots.flatMap((pot, idx) => [
    { name: `${pot.id}__in`, label: `${pot.name} +`, color: palette.categorical[idx], stackId: 'inflow' },
    { name: `${pot.id}__out`, label: `${pot.name} −`, color: palette.categorical[idx], stackId: 'outflow' },
  ])
  const legendSeries = selectedPots.map((pot, idx) => ({ name: String(pot.id), label: pot.name, color: palette.categorical[idx] }))

  const values = data.flatMap((row) => series.map((s) => Number(row[s.name]) || 0))
  const ticks = niceAxisTicksSigned(Math.min(...values, 0), Math.max(...values, 0))
  const xInterval = categoryTickInterval(data.length)

  // A visible gap between stacked segments, per the dataviz standard —
  // stroked in the surface color so adjacent segments never visually merge.
  const segmentGap = { strokeWidth: 2, stroke: 'var(--mantine-color-body)' }

  return (
    <>
      {legendSeries.length > 1 && (
        <div style={{ flexShrink: 0 }}>
          <ChartLegend series={legendSeries} />
        </div>
      )}
      <div style={{ flex: 1, minHeight: 0 }}>
        <BarChart
          h="100%"
          data={data}
          dataKey="month"
          withLegend={false}
          valueFormatter={(v: number) => formatCents(Math.round(v * 100), locale)}
          series={series}
          barProps={() => segmentGap}
          xAxisProps={{ interval: xInterval }}
          yAxisProps={{
            tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale),
            width: 56,
            ticks,
            domain: [ticks[0], ticks[ticks.length - 1]],
          }}
          referenceLines={[{ y: 0, color: 'gray.5' }]}
        />
      </div>
    </>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(PotFlowWidget)
