import { memo } from 'react'
import { Skeleton, Text } from '@mantine/core'
import { BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import { useTrendsDescriptions } from '../../api/hooks/usePeriods'
import { formatCents, formatCentsCompact } from '../../lib/money'
import { niceAxisTicks } from '../../lib/chartAxis'
import { useChartPalette } from '../../contexts/ChartPaletteContext'

interface Props {
  categoryId: number | null
  limit: number
}

const MAX_LABEL_CHARS = 16

function truncate(label: string): string {
  return label.length > MAX_LABEL_CHARS ? `${label.slice(0, MAX_LABEL_CHARS - 1)}…` : label
}

// Self-contained, like ItemizedIncomeWidget — the category and row limit are
// the only configurable fields, so there's no page-level filter for this one
// to hook into; it fetches its own ranked breakdown.
function TopDescriptionsWidget({ categoryId, limit }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()
  const { data, isLoading, error } = useTrendsDescriptions({ categoryId: categoryId ?? undefined, limit })

  if (isLoading) return <Skeleton h="100%" />
  // A failed request is not the same as an empty result: without this the
  // widget told the user there was nothing to show when the call had in
  // fact errored.
  if (error) return <Text size="sm" c="dimmed">{t('common.error')}</Text>
  if (!data || data.length === 0) return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>

  const rows = [...data].sort((a, b) => b.totalCents - a.totalCents)
  // Long descriptions are free text ("Boodschappen Albert Heijn weekend");
  // left whole they wrap onto a second line inside the fixed y-axis width
  // and the neighbouring rows' labels then collide. The full text stays in
  // the tooltip.
  const chartData = rows.map((row) => ({
    description: `${truncate(row.description)} (${row.count}×)`,
    amount: row.totalCents / 100,
  }))
  const maxValue = Math.max(...chartData.map((row) => row.amount), 0)
  const ticks = niceAxisTicks(maxValue)

  return (
    <div style={{ flex: 1, minHeight: 0 }}>
      <BarChart
        h="100%"
        data={chartData}
        dataKey="description"
        orientation="vertical"
        withLegend={false}
        valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
        series={[{ name: 'amount', label: t('trends.amount'), color: palette.categorical[0] }]}
        xAxisProps={{ tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale), ticks, domain: [0, ticks[ticks.length - 1]] }}
        // interval 0: a category axis must label every bar — recharts' default
        // thinning silently drops labels, which read as bars belonging to the
        // row above them.
        yAxisProps={{ width: 150, interval: 0 }}
        maxBarWidth={28}
      />
    </div>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(TopDescriptionsWidget)
