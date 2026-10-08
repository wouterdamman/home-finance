import { memo } from 'react'
import { Skeleton, Text, Center } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { ResponsiveContainer, Sankey, Tooltip, Rectangle } from 'recharts'
import type { TooltipContentProps } from 'recharts'
import type { SankeyNodeProps, SankeyLinkProps } from 'recharts'
import { useTrendsIncomeSources, useTrendsCategoryTotals, useTrendsPotBalances, useTrendsMonthlyTotals } from '../../api/hooks/usePeriods'
import { formatCents } from '../../lib/money'
import { useChartPalette } from '../../contexts/ChartPaletteContext'
import { paletteColorValue } from '../../lib/chartPalette'
import { incomeSourceLabel } from '../../lib/incomeSourceLabel'
import { UNCATEGORIZED_CATEGORY_ID } from '../../lib/categoryLabel'

interface Props {
  year: number
}

// Layer tags on each node's payload — used by the custom node renderer to
// decide which side of the rectangle its label sits on (left column labels
// sit to the left, the hub's label sits above, right column labels sit to
// the right), and by the custom link renderer to pick the "real entity"
// endpoint's color (a link touching the hub is colored by its non-hub end,
// never by the hub itself).
type SankeyLayer = 0 | 1 | 2

interface FlowNode {
  name: string
  color: string
  layer: SankeyLayer
}

interface FlowLink {
  source: number
  target: number
  value: number
}

const MAX_LABEL_CHARS = 14
// Sources/categories/pots below this share of their layer's total fold into
// one explicit "Other" node — same idea as IncomeMixWidget's small-share
// fold, just applied to three node layers instead of one donut.
const SMALL_SHARE_THRESHOLD = 0.03
const MAX_REAL_NODES_PER_SIDE = 7
// "No category" is a different kind of bucket than "Other categories" (it's
// budget lines with no category at all, not a fold of small real ones) —
// neutral gray so it never reads as competing for a categorical slot.
const OTHER_CATEGORIES_COLOR = 'gray.5'
const UNCATEGORIZED_COLOR = 'gray.7'
const OTHER_SOURCES_COLOR = 'gray.5'
// Pots are a different kind of destination from expense categories — money
// kept, not money spent — so they share one dedicated hue instead of
// drawing from the categorical slots. Those slots are already taken twice
// over (income sources count up from 0, categories count down from 6), so
// reusing them put a pot and a category in the same column in the same
// colour; the pots' own labels tell them apart.
const POT_COLOR = 'teal.7'

function truncate(label: string): string {
  return label.length > MAX_LABEL_CHARS ? `${label.slice(0, MAX_LABEL_CHARS - 1)}…` : label
}

function foldSmallEntries(
  entries: { name: string; cents: number }[],
): { kept: { name: string; cents: number }[]; otherCents: number } {
  const sorted = [...entries].filter((e) => e.cents > 0).sort((a, b) => b.cents - a.cents)
  const total = sorted.reduce((sum, e) => sum + e.cents, 0)
  if (total <= 0) return { kept: [], otherCents: 0 }
  const big = sorted.filter((e) => e.cents / total >= SMALL_SHARE_THRESHOLD)
  const kept = big.slice(0, MAX_REAL_NODES_PER_SIDE)
  const otherCents = total - kept.reduce((sum, e) => sum + e.cents, 0)
  return { kept, otherCents }
}

function SankeyTooltip({ active, payload, locale }: TooltipContentProps & { locale: string }) {
  if (!active || !payload || payload.length === 0) return null
  const entry = payload[0]
  const value = typeof entry.value === 'number' ? entry.value : 0
  return (
    <div style={{ background: 'var(--mantine-color-body)', border: '1px solid var(--mantine-color-default-border)', borderRadius: 4, padding: '4px 8px', fontSize: 12 }}>
      <Text size="xs" fw={600}>{entry.name}</Text>
      <Text size="xs">{formatCents(Math.round(value * 100), locale)}</Text>
    </div>
  )
}

function renderNode(props: SankeyNodeProps) {
  const { x, y, width, height, index, payload } = props
  const node = payload as unknown as FlowNode
  const fill = paletteColorValue(node.color)
  const isLeft = node.layer === 0
  const isHub = node.layer === 1
  const labelX = isLeft ? x - 6 : isHub ? x + width / 2 : x + width + 6
  const labelY = isHub ? y - 8 : y + height / 2
  const textAnchor = isLeft ? 'end' : isHub ? 'middle' : 'start'
  return (
    <g key={`sankey-node-${index}`}>
      <Rectangle x={x} y={y} width={width} height={height} fill={fill} fillOpacity={0.9} />
      <text x={labelX} y={labelY} dy={isHub ? 0 : 4} textAnchor={textAnchor} fontSize={12} style={{ fill: 'var(--mantine-color-text)' }}>
        {node.name}
      </text>
    </g>
  )
}

function renderLink(props: SankeyLinkProps) {
  const { sourceX, targetX, sourceY, targetY, sourceControlX, targetControlX, linkWidth, index, payload } = props
  const source = payload.source as unknown as FlowNode
  const target = payload.target as unknown as FlowNode
  // A link touching the hub is colored by whichever end is the real entity
  // (source on the way in, target on the way out) — never by the hub's own
  // neutral color, so the flow keeps reading as "that source"/"that
  // category/pot" all the way through.
  const entity = source.layer === 1 ? target : source
  return (
    <path
      key={`sankey-link-${index}`}
      d={`M${sourceX},${sourceY} C${sourceControlX},${sourceY} ${targetControlX},${targetY} ${targetX},${targetY}`}
      fill="none"
      stroke={paletteColorValue(entity.color)}
      strokeWidth={linkWidth}
      strokeOpacity={0.35}
    />
  )
}

// Self-contained, own year like monthCompare/allTimeTrend/savingsRate —
// independent of the page's global year filter. Not wired through
// Trends.tsx's shared catData/incomeData/potData props because those
// queries already return every year's data and this widget needs all
// three plus monthly-totals (for the uncategorised gap) regardless.
function SankeyFlowWidget({ year }: Props) {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()
  // A Sankey with three columns of labeled nodes doesn't have room to
  // reflow below ~600px — rendering it squashed is unreadable, so a short
  // explanatory message replaces the diagram instead.
  const isNarrow = useMediaQuery('(max-width: 37.4em)')

  const incomeQuery = useTrendsIncomeSources()
  const catQuery = useTrendsCategoryTotals()
  const potQuery = useTrendsPotBalances()
  const monthlyQuery = useTrendsMonthlyTotals()

  if (incomeQuery.isLoading || catQuery.isLoading || potQuery.isLoading || monthlyQuery.isLoading) {
    return <Skeleton h="100%" />
  }
  const incomeData = incomeQuery.data
  const catData = catQuery.data
  const potData = potQuery.data
  const monthlyTotals = monthlyQuery.data
  if (!incomeData || !catData || !potData || !monthlyTotals) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  if (isNarrow) {
    // Centered rather than top-aligned: the widget keeps its configured
    // 4-row height on mobile, and a line of text pinned to the top of that
    // much empty card reads as a half-loaded chart.
    return (
      <Center style={{ flex: 1, minHeight: 0 }}>
        <Text size="sm" c="dimmed" ta="center">{t('trends.sankeyMobileMessage')}</Text>
      </Center>
    )
  }

  const sourceEntries = incomeData.sources.map((s) => ({
    name: incomeSourceLabel(s, t),
    cents: incomeData.entries.filter((e) => e.year === year).reduce((sum, e) => sum + (e.values[String(s.id)] ?? 0), 0),
  }))
  const { kept: keptSources, otherCents: otherSourceCents } = foldSmallEntries(sourceEntries)

  // The synthetic id-0 "Uncategorised" bucket (label-only budget lines) is
  // excluded here — it gets its own gray node below, derived from the gap
  // against yearExpenseTotalCents, same as CategoryShareWidget. Including
  // it in categoryEntries too would double-count it (once as a normal
  // categorical-colored node, once as the gap-derived gray one).
  const categoryEntries = catData.categories
    .filter((c) => c.id !== UNCATEGORIZED_CATEGORY_ID)
    .map((c) => ({
      name: c.name,
      cents: catData.entries.filter((e) => e.year === year).reduce((sum, e) => sum + (e.values[String(c.id)] ?? 0), 0),
    }))
  const { kept: keptCategories, otherCents: otherCategoryCents } = foldSmallEntries(categoryEntries)
  const categoriesSum = categoryEntries.reduce((sum, c) => sum + c.cents, 0)
  const yearExpenseTotalCents = monthlyTotals.filter((m) => m.year === year).reduce((sum, m) => sum + m.expenseTotalCents, 0)
  // Same gap as CategoryShareWidget: the real categories' own sum
  // under-reports the real year expense total by exactly the label-only
  // budget lines' amount (plus any rounding/timing drift between the two
  // independent queries).
  const uncategorizedCents = Math.max(0, yearExpenseTotalCents - categoriesSum)

  const potEntries = potData.pots.map((p) => ({
    name: p.name,
    cents: potData.entries.filter((e) => e.year === year).reduce((sum, e) => sum + (e.inflow[String(p.id)] ?? 0), 0),
  })).filter((p) => p.cents > 0).sort((a, b) => b.cents - a.cents).slice(0, MAX_REAL_NODES_PER_SIDE)

  const incomeTotalCents = sourceEntries.reduce((sum, s) => sum + s.cents, 0)

  if (incomeTotalCents <= 0 && yearExpenseTotalCents <= 0) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  const nodes: FlowNode[] = []
  const links: FlowLink[] = []

  const sourceStartIdx = nodes.length
  keptSources.forEach((s, idx) => nodes.push({ name: truncate(s.name), color: palette.categorical[idx], layer: 0 }))
  const otherSourceIdx = otherSourceCents > 0 ? nodes.push({ name: t('trends.otherSource'), color: OTHER_SOURCES_COLOR, layer: 0 }) - 1 : -1

  const hubIdx = nodes.push({ name: t('trends.incomeHubLabel'), color: palette.surplus, layer: 1 }) - 1

  keptSources.forEach((s, idx) => links.push({ source: sourceStartIdx + idx, target: hubIdx, value: s.cents / 100 }))
  if (otherSourceIdx >= 0) links.push({ source: otherSourceIdx, target: hubIdx, value: otherSourceCents / 100 })

  keptCategories.forEach((c, idx) => {
    // Reversed slot order from the income-source side (slot 6 down to 0)
    // so the first category never shares an income source's exact hue in
    // the same diagram — still a fixed, deterministic position, never a
    // cycled/random hue.
    const categoryIdx = nodes.push({ name: truncate(c.name), color: palette.categorical[MAX_REAL_NODES_PER_SIDE - 1 - idx], layer: 2 }) - 1
    links.push({ source: hubIdx, target: categoryIdx, value: c.cents / 100 })
  })
  if (otherCategoryCents > 0) {
    const idx = nodes.push({ name: t('trends.otherCategories'), color: OTHER_CATEGORIES_COLOR, layer: 2 }) - 1
    links.push({ source: hubIdx, target: idx, value: otherCategoryCents / 100 })
  }
  if (uncategorizedCents > 0) {
    const idx = nodes.push({ name: t('trends.uncategorizedCategory'), color: UNCATEGORIZED_COLOR, layer: 2 }) - 1
    links.push({ source: hubIdx, target: idx, value: uncategorizedCents / 100 })
  }
  potEntries.forEach((p) => {
    const potIdx = nodes.push({ name: truncate(p.name), color: POT_COLOR, layer: 2 }) - 1
    links.push({ source: hubIdx, target: potIdx, value: p.cents / 100 })
  })

  if (links.length === 0) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  return (
    <div style={{ flex: 1, minHeight: 0 }}>
      <ResponsiveContainer width="100%" height="100%">
        <Sankey
          data={{ nodes, links }}
          node={renderNode}
          link={renderLink}
          nodePadding={20}
          nodeWidth={10}
          // Side margins hold the node labels, which recharts draws outside
          // the plot area: they must cover MAX_LABEL_CHARS at 12px plus the
          // 6px offset in renderNode, or the right column's text is clipped
          // by the card edge.
          margin={{ top: 10, right: 132, bottom: 10, left: 132 }}
        >
          <Tooltip content={(props) => <SankeyTooltip {...props} locale={locale} />} />
        </Sankey>
      </ResponsiveContainer>
    </div>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(SankeyFlowWidget)
