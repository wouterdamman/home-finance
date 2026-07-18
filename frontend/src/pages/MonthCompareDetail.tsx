import { useMemo, useState } from 'react'
import { Title, Skeleton, Alert, Text, Stack, Group, SimpleGrid, MultiSelect, Paper, Table, TextInput, ActionIcon, Tooltip, Anchor, Box } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { useQueries } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { CompositeChart } from '@mantine/charts'
import { IconSearch, IconEyeOff, IconChevronUp, IconChevronDown, IconSelector } from '@tabler/icons-react'
import { api } from '../api/client'
import type { YearSummary } from '../api/types'
import { useTrendsCategoryTotals } from '../api/hooks/usePeriods'
import { formatCents, formatCentsCompact } from '../lib/money'
import { niceAxisTicks } from '../lib/chartAxis'
import ChartLegend from '../components/trends/ChartLegend'
import { MobileListRow } from '../components/mobile/MobileList'
import EmptyState from '../components/EmptyState'

const MONTHS_NL = ['jan', 'feb', 'mrt', 'apr', 'mei', 'jun', 'jul', 'aug', 'sep', 'okt', 'nov', 'dec']
const MONTHS_EN = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const MAX_PERIODS = 4
const MIN_PERIODS = 2

type SortKey = 'name' | 'trend' | number

interface Period {
  year: number
  month: number
}

function periodKey(p: Period): string {
  return `${p.year}-${p.month}`
}

function parsePeriodKey(key: string): Period {
  const [year, month] = key.split('-').map(Number)
  return { year, month }
}

function trendColor(diff: number, higherIsBad: boolean): string {
  if (diff === 0) return 'dimmed'
  const bad = higherIsBad ? diff > 0 : diff < 0
  return bad ? 'red' : 'green'
}

function TrendText({ diff, locale, higherIsBad }: { diff: number; locale: string; higherIsBad: boolean }) {
  return (
    <Text size="sm" fw={600} c={trendColor(diff, higherIsBad)} span>
      {diff >= 0 ? '+' : ''}{formatCents(diff, locale)}
    </Text>
  )
}

function SortableTh({ label, active, dir, align, onClick }: { label: string; active: boolean; dir: 'asc' | 'desc'; align?: 'right'; onClick: () => void }) {
  const Icon = !active ? IconSelector : dir === 'asc' ? IconChevronUp : IconChevronDown
  return (
    <Table.Th ta={align} style={{ cursor: 'pointer', userSelect: 'none' }} onClick={onClick}>
      <Group gap={4} justify={align === 'right' ? 'flex-end' : 'flex-start'} wrap="nowrap">
        <Text fw={700} size="sm">{label}</Text>
        <Icon size={14} style={{ opacity: active ? 1 : 0.4, flexShrink: 0 }} />
      </Group>
    </Table.Th>
  )
}

// Income/expenses (bars) + surplus (line) across the selected periods —
// the same bar+bar+line convention as the year-dashboard overview chart,
// scoped to just the periods being compared here.
function TotalsTrendChart({ periodLabels, totalsRows, locale }: { periodLabels: string[]; totalsRows: { label: string; values: number[]; higherIsBad: boolean }[]; locale: string }) {
  const { t } = useTranslation()
  const income = totalsRows.find((r) => r.label === t('year.income'))?.values ?? []
  const expenses = totalsRows.find((r) => r.label === t('year.expenses'))?.values ?? []
  const surplus = totalsRows.find((r) => r.label === t('year.surplus'))?.values ?? []
  const chartData = periodLabels.map((label, i) => ({
    period: label,
    [t('year.income')]: (income[i] ?? 0) / 100,
    [t('year.expenses')]: (expenses[i] ?? 0) / 100,
    [t('year.surplus')]: (surplus[i] ?? 0) / 100,
  }))
  const series = [
    { name: t('year.income'), color: 'teal.6', type: 'bar' as const },
    { name: t('year.expenses'), color: 'red.6', type: 'bar' as const },
    { name: t('year.surplus'), color: 'blue.6', type: 'line' as const },
  ]
  const maxValue = Math.max(...chartData.flatMap((row) => series.map((s) => Number(row[s.name]) || 0)), 0)
  const ticks = niceAxisTicks(maxValue)
  return (
    <Paper withBorder p="md">
      <div style={{ flexShrink: 0 }}>
        <ChartLegend series={series.map((s) => ({ name: s.name, color: s.color }))} />
      </div>
      <CompositeChart
        h={220}
        data={chartData}
        dataKey="period"
        withLegend={false}
        valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
        yAxisProps={{ tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale), width: 56, ticks, domain: [0, ticks[ticks.length - 1]] }}
        series={series}
      />
    </Paper>
  )
}

// Simple horizontal ranking bars — a magnitude read (dataviz: sequential,
// single hue) of which categories moved most between the first and last
// selected period, ahead of the full numeric table below it.
function BiggestMoversChart({ rows, locale }: { rows: { name: string; trend: number }[]; locale: string }) {
  const top = [...rows].sort((a, b) => Math.abs(b.trend) - Math.abs(a.trend)).slice(0, 8)
  const maxAbs = Math.max(...top.map((r) => Math.abs(r.trend)), 1)
  const { t } = useTranslation()
  return (
    <Paper withBorder p="md">
      <Text size="sm" fw={600} mb="sm">{t('trends.biggestMovers')}</Text>
      <Stack gap={6}>
        {top.map((row) => {
          const pct = Math.round((Math.abs(row.trend) / maxAbs) * 100)
          const bad = row.trend > 0
          return (
            <Group key={row.name} gap="sm" wrap="nowrap">
              <Text size="xs" style={{ width: 140, flexShrink: 0 }} truncate>{row.name}</Text>
              <Box style={{ flex: 1, height: 8, background: 'var(--mantine-color-default-border)', borderRadius: 4, overflow: 'hidden' }}>
                <Box style={{ width: `${pct}%`, height: '100%', background: `var(--mantine-color-${bad ? 'red' : 'teal'}-6)`, borderRadius: 4 }} />
              </Box>
              <Text size="xs" c={bad ? 'red' : 'teal'} fw={600} style={{ width: 90, textAlign: 'right', flexShrink: 0 }}>
                {row.trend >= 0 ? '+' : ''}{formatCents(row.trend, locale)}
              </Text>
            </Group>
          )
        })}
      </Stack>
    </Paper>
  )
}

export default function MonthCompareDetail() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const monthNames = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const isMobile = useMediaQuery('(max-width: 47.99em)')

  const categoryQuery = useTrendsCategoryTotals()

  const availablePeriods = useMemo<Period[]>(
    () => (categoryQuery.data?.entries ?? []).map((e) => ({ year: e.year, month: e.month })),
    [categoryQuery.data],
  )

  const [selectedKeys, setSelectedKeys] = useState<string[] | null>(null)
  const [search, setSearch] = useState('')
  const [hiddenIds, setHiddenIds] = useState<Set<number>>(new Set())
  const [sortKey, setSortKey] = useState<SortKey>('trend')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')

  const effectiveKeys = selectedKeys ?? availablePeriods.slice(-3).map(periodKey)
  const periods = effectiveKeys.map(parsePeriodKey).sort((a, b) => (a.year - b.year) || (a.month - b.month))
  const uniqueYears = [...new Set(periods.map((p) => p.year))]

  const summaryResults = useQueries({
    queries: uniqueYears.map((year) => ({
      queryKey: ['year-summary', year],
      queryFn: () => api.get<YearSummary>(`/api/years/${year}/summary`),
    })),
  })
  const summaryByYear = new Map(uniqueYears.map((year, i) => [year, summaryResults[i]]))

  if (categoryQuery.isLoading || summaryResults.some((r) => r.isLoading)) {
    return <Skeleton h={400} />
  }
  if (categoryQuery.error || summaryResults.some((r) => r.error)) return <Alert color="red">{t('common.error')}</Alert>
  if (!categoryQuery.data) return null

  const periodOptions = availablePeriods.map((p) => ({
    value: periodKey(p),
    label: `${monthNames[p.month - 1]} ${p.year}`,
  }))

  if (periodOptions.length === 0) {
    return (
      <Stack gap="xl">
        <Text component={Link} to="/trends" c="blue" size="sm">{t('trends.back')}</Text>
        <Title order={2}>{t('trends.compareMonthsTitle')}</Title>
        <EmptyState message={t('trends.noData')} />
      </Stack>
    )
  }

  const periodLabels = periods.map((p) => `${monthNames[p.month - 1]} ${p.year}`)
  const monthRows = periods.map((p) => summaryByYear.get(p.year)?.data?.months[p.month - 1])
  const entries = periods.map((p) => categoryQuery.data!.entries.find((e) => e.year === p.year && e.month === p.month))

  const enoughPeriods = periods.length >= MIN_PERIODS

  const searchLower = search.trim().toLowerCase()
  const sortMultiplier = sortDir === 'asc' ? 1 : -1

  const categoryRows = categoryQuery.data.categories
    .map((cat) => {
      const values = entries.map((e) => e?.values[String(cat.id)] ?? 0)
      const trend = values.length >= 2 ? values[values.length - 1] - values[0] : 0
      return { id: cat.id, name: cat.name, values, trend }
    })
    .filter((row) => !hiddenIds.has(row.id))
    .filter((row) => searchLower === '' || row.name.toLowerCase().includes(searchLower))
    .sort((a, b) => {
      if (sortKey === 'name') return a.name.localeCompare(b.name) * sortMultiplier
      if (sortKey === 'trend') return (Math.abs(a.trend) - Math.abs(b.trend)) * sortMultiplier
      return (a.values[sortKey] - b.values[sortKey]) * sortMultiplier
    })

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir(key === 'name' ? 'asc' : 'desc')
    }
  }

  const toggleHidden = (id: number) => {
    setHiddenIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const totalsRows: { label: string; values: number[]; higherIsBad: boolean }[] = enoughPeriods
    ? [
        { label: t('year.income'), values: monthRows.map((m) => m?.incomeTotalCents ?? 0), higherIsBad: false },
        { label: t('year.expenses'), values: monthRows.map((m) => m?.expenseTotalCents ?? 0), higherIsBad: true },
        { label: t('year.surplus'), values: monthRows.map((m) => m?.surplusCents ?? 0), higherIsBad: false },
      ]
    : []

  return (
    <Stack gap="xl">
      <Text component={Link} to="/trends" c="blue" size="sm">{t('trends.back')}</Text>
      <Title order={isMobile ? 3 : 2}>{t('trends.compareMonthsTitle')}</Title>

      <MultiSelect
        label={t('trends.periods')}
        data={periodOptions}
        value={effectiveKeys}
        onChange={setSelectedKeys}
        maxValues={MAX_PERIODS}
        w={isMobile ? '100%' : 480}
      />
      {!enoughPeriods && <Text size="sm" c="dimmed">{t('trends.selectAtLeastTwoPeriods')}</Text>}

      {enoughPeriods && (
        <>
          <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
            <Table.ScrollContainer minWidth={360}>
              <Paper withBorder p="md" h="100%">
                <Table>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th></Table.Th>
                      {periodLabels.map((label) => <Table.Th key={label} ta="right">{label}</Table.Th>)}
                      <Table.Th ta="right">{t('trends.trend')}</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {totalsRows.map((row) => (
                      <Table.Tr key={row.label}>
                        <Table.Td><Text size="sm" c="dimmed">{row.label}</Text></Table.Td>
                        {row.values.map((v, i) => <Table.Td key={i} ta="right">{formatCents(v, locale)}</Table.Td>)}
                        <Table.Td ta="right"><TrendText diff={row.values[row.values.length - 1] - row.values[0]} locale={locale} higherIsBad={row.higherIsBad} /></Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              </Paper>
            </Table.ScrollContainer>

            <TotalsTrendChart periodLabels={periodLabels} totalsRows={totalsRows} locale={locale} />
          </SimpleGrid>

          <BiggestMoversChart rows={categoryRows} locale={locale} />

          <Group justify="space-between" wrap="wrap" align="center">
            <TextInput
              placeholder={t('trends.searchCategories')}
              leftSection={<IconSearch size={16} />}
              value={search}
              onChange={(e) => setSearch(e.currentTarget.value)}
              w={isMobile ? '100%' : 280}
            />
            {hiddenIds.size > 0 && (
              <Anchor size="sm" onClick={() => setHiddenIds(new Set())}>
                {t('trends.showHiddenCategories', { count: hiddenIds.size })}
              </Anchor>
            )}
          </Group>

          {isMobile ? (
            <Stack gap={0}>
              {categoryRows.map((row) => (
                <MobileListRow
                  key={row.id}
                  title={row.name}
                  subtitle={row.values.map((v) => formatCents(v, locale)).join(' → ')}
                  trailing={
                    <Group gap={8} wrap="nowrap">
                      <TrendText diff={row.trend} locale={locale} higherIsBad />
                      <ActionIcon variant="subtle" color="gray" size="sm" aria-label={t('trends.hideCategory')} onClick={() => toggleHidden(row.id)}>
                        <IconEyeOff size={16} />
                      </ActionIcon>
                    </Group>
                  }
                />
              ))}
            </Stack>
          ) : (
            <Table.ScrollContainer minWidth={520}>
              <Table striped>
                <Table.Thead>
                  <Table.Tr>
                    <SortableTh label={t('settings.categories')} active={sortKey === 'name'} dir={sortDir} onClick={() => toggleSort('name')} />
                    {periodLabels.map((label, i) => (
                      <SortableTh key={label} label={label} active={sortKey === i} dir={sortDir} align="right" onClick={() => toggleSort(i)} />
                    ))}
                    <SortableTh label={t('trends.trend')} active={sortKey === 'trend'} dir={sortDir} align="right" onClick={() => toggleSort('trend')} />
                    <Table.Th w={40} />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {categoryRows.map((row) => (
                    <Table.Tr key={row.id}>
                      <Table.Td>{row.name}</Table.Td>
                      {row.values.map((v, i) => <Table.Td key={i} ta="right">{formatCents(v, locale)}</Table.Td>)}
                      <Table.Td ta="right"><TrendText diff={row.trend} locale={locale} higherIsBad /></Table.Td>
                      <Table.Td>
                        <Tooltip label={t('trends.hideCategory')}>
                          <ActionIcon variant="subtle" color="gray" size="sm" aria-label={t('trends.hideCategory')} onClick={() => toggleHidden(row.id)}>
                            <IconEyeOff size={16} />
                          </ActionIcon>
                        </Tooltip>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
        </>
      )}
    </Stack>
  )
}
