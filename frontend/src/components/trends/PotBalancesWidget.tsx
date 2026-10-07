import { memo } from 'react'
import { AreaChart, BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import type { TrendsFilter } from '../../lib/trendsFilter'
import type { TrendsPotBalances } from '../../api/types'
import type { ChartKind } from '../../lib/trendsDashboard'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicks, categoryTickInterval } from '../../lib/chartAxis'
import { monthNames } from '../../lib/monthNames'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import ChartLegend from './ChartLegend'

interface Props {
  potIds: number[]
  chartKind: ChartKind
  filter: TrendsFilter
  potData: TrendsPotBalances
}

// 'bar' renders a stacked BarChart; 'line' renders a stacked AreaChart —
// same convention as IncomeSourcesWidget (a set of per-pot lines wouldn't
// stack, and stacking is the point here too).
function PotBalancesWidget({ potIds, chartKind, filter, potData }: Props) {
  const { i18n } = useTranslation()
  const months = monthNames(i18n.language)
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  const selectedPots = potIds
    .map((id) => potData.pots.find((p) => p.id === id))
    .filter((p): p is NonNullable<typeof p> => p != null)

  // Balances are a closing-balance carry-forward, not a flow — a month with
  // no pot ledger mutations at all still has to render flat, never dropping
  // to zero. If an entry for a given month is ever missing outright, carry
  // the previous month's balance forward instead of defaulting to 0.
  const lastBalance: Record<string, number> = {}
  const data = months.map((label, i) => {
    const entry = potData.entries.find((e) => e.year === filter.year && e.month === i + 1)
    const row: Record<string, string | number> = { month: label }
    for (const pot of selectedPots) {
      const key = String(pot.id)
      const raw = entry?.balances[key]
      const cents = raw != null ? raw : (lastBalance[key] ?? 0)
      lastBalance[key] = cents
      // Keyed by id, labelled by name: pot names are not unique (an
      // archived pot and its recreated namesake both appear here), and a
      // name key would silently overwrite one series with the other.
      row[key] = cents / 100
    }
    return row
  })

  const series = selectedPots.map((pot, idx) => ({ name: String(pot.id), label: pot.name, color: palette.categorical[idx] }))
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
export default memo(PotBalancesWidget)
