import { useMemo, useState } from 'react'
import { Title, Skeleton, Alert, Text, Stack, Group, Select, Paper, Table, Divider, TextInput, ActionIcon, Tooltip, Anchor } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { IconSearch, IconEyeOff, IconChevronUp, IconChevronDown, IconSelector } from '@tabler/icons-react'
import { useTrendsCategoryTotals, useYearSummary } from '../api/hooks/usePeriods'
import { formatCents } from '../lib/money'
import { MobileListRow } from '../components/mobile/MobileList'
import EmptyState from '../components/EmptyState'

const MONTHS_NL = ['jan', 'feb', 'mrt', 'apr', 'mei', 'jun', 'jul', 'aug', 'sep', 'okt', 'nov', 'dec']
const MONTHS_EN = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

type SortKey = 'name' | 'a' | 'b' | 'diff'
type SortDir = 'asc' | 'desc'

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

function TotalsRow({ label, aCents, bCents, higherIsBad }: { label: string; aCents: number; bCents: number; higherIsBad: boolean }) {
  const { i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const diff = bCents - aCents
  const diffBad = higherIsBad ? diff > 0 : diff < 0
  const diffColor = diff === 0 ? 'dimmed' : diffBad ? 'red' : 'green'
  return (
    <Group justify="space-between" wrap="wrap">
      <Text size="sm" c="dimmed">{label}</Text>
      <Group gap={6} wrap="nowrap">
        <Text size="sm">{formatCents(aCents, locale)}</Text>
        <Text size="sm" c="dimmed">→</Text>
        <Text size="sm">{formatCents(bCents, locale)}</Text>
        <Text size="sm" c={diffColor} fw={600}>
          ({diff >= 0 ? '+' : ''}{formatCents(diff, locale)})
        </Text>
      </Group>
    </Group>
  )
}

function SortableTh({ label, active, dir, align, onClick }: { label: string; active: boolean; dir: SortDir; align?: 'right'; onClick: () => void }) {
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

  const [periodAKey, setPeriodAKey] = useState<string | null>(null)
  const [periodBKey, setPeriodBKey] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [hiddenIds, setHiddenIds] = useState<Set<number>>(new Set())
  const [sortKey, setSortKey] = useState<SortKey>('diff')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  const effectiveAKey = periodAKey ?? (availablePeriods.length >= 2 ? periodKey(availablePeriods[availablePeriods.length - 2]) : availablePeriods[0] ? periodKey(availablePeriods[0]) : null)
  const effectiveBKey = periodBKey ?? (availablePeriods.length >= 1 ? periodKey(availablePeriods[availablePeriods.length - 1]) : null)

  const periodA = effectiveAKey ? parsePeriodKey(effectiveAKey) : null
  const periodB = effectiveBKey ? parsePeriodKey(effectiveBKey) : null

  const summaryAQuery = useYearSummary(periodA?.year ?? 0)
  const summaryBQuery = useYearSummary(periodB?.year ?? 0)

  if (categoryQuery.isLoading || (periodA && summaryAQuery.isLoading) || (periodB && summaryBQuery.isLoading)) {
    return <Skeleton h={400} />
  }
  if (categoryQuery.error || summaryAQuery.error || summaryBQuery.error) return <Alert color="red">{t('common.error')}</Alert>
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

  const monthA = periodA && summaryAQuery.data ? summaryAQuery.data.months[periodA.month - 1] : null
  const monthB = periodB && summaryBQuery.data ? summaryBQuery.data.months[periodB.month - 1] : null

  const entryA = periodA ? categoryQuery.data.entries.find((e) => e.year === periodA.year && e.month === periodA.month) : undefined
  const entryB = periodB ? categoryQuery.data.entries.find((e) => e.year === periodB.year && e.month === periodB.month) : undefined

  const searchLower = search.trim().toLowerCase()
  const sortMultiplier = sortDir === 'asc' ? 1 : -1

  const categoryRows = categoryQuery.data.categories
    .map((cat) => {
      const aCents = entryA?.values[String(cat.id)] ?? 0
      const bCents = entryB?.values[String(cat.id)] ?? 0
      return { id: cat.id, name: cat.name, aCents, bCents, diff: bCents - aCents }
    })
    .filter((row) => !hiddenIds.has(row.id))
    .filter((row) => searchLower === '' || row.name.toLowerCase().includes(searchLower))
    .sort((a, b) => {
      if (sortKey === 'name') return a.name.localeCompare(b.name) * sortMultiplier
      if (sortKey === 'a') return (a.aCents - b.aCents) * sortMultiplier
      if (sortKey === 'b') return (a.bCents - b.bCents) * sortMultiplier
      return (Math.abs(a.diff) - Math.abs(b.diff)) * sortMultiplier
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

  const periodALabel = periodOptions.find((o) => o.value === effectiveAKey)?.label ?? ''
  const periodBLabel = periodOptions.find((o) => o.value === effectiveBKey)?.label ?? ''

  return (
    <Stack gap="xl">
      <Text component={Link} to="/trends" c="blue" size="sm">{t('trends.back')}</Text>
      <Title order={isMobile ? 3 : 2}>{t('trends.compareMonthsTitle')}</Title>

      <Group grow wrap={isMobile ? 'wrap' : 'nowrap'}>
        <Select
          label={t('trends.periodA')}
          data={periodOptions}
          value={effectiveAKey}
          onChange={setPeriodAKey}
          allowDeselect={false}
        />
        <Select
          label={t('trends.periodB')}
          data={periodOptions}
          value={effectiveBKey}
          onChange={setPeriodBKey}
          allowDeselect={false}
        />
      </Group>

      {monthA && monthB ? (
        <Paper withBorder p="md">
          <Stack gap="xs">
            <TotalsRow label={t('year.income')} aCents={monthA.incomeTotalCents} bCents={monthB.incomeTotalCents} higherIsBad={false} />
            <TotalsRow label={t('year.expenses')} aCents={monthA.expenseTotalCents} bCents={monthB.expenseTotalCents} higherIsBad />
            <Divider />
            <TotalsRow label={t('year.surplus')} aCents={monthA.surplusCents} bCents={monthB.surplusCents} higherIsBad={false} />
          </Stack>
        </Paper>
      ) : null}

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
              subtitle={`${formatCents(row.aCents, locale)} → ${formatCents(row.bCents, locale)}`}
              trailing={
                <Group gap={8} wrap="nowrap">
                  <Text size="sm" fw={600} c={row.diff === 0 ? 'dimmed' : row.diff > 0 ? 'red' : 'green'}>
                    {row.diff >= 0 ? '+' : ''}{formatCents(row.diff, locale)}
                  </Text>
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
                <SortableTh label={periodALabel} active={sortKey === 'a'} dir={sortDir} align="right" onClick={() => toggleSort('a')} />
                <SortableTh label={periodBLabel} active={sortKey === 'b'} dir={sortDir} align="right" onClick={() => toggleSort('b')} />
                <SortableTh label="Δ" active={sortKey === 'diff'} dir={sortDir} align="right" onClick={() => toggleSort('diff')} />
                <Table.Th w={40} />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {categoryRows.map((row) => (
                <Table.Tr key={row.id}>
                  <Table.Td>{row.name}</Table.Td>
                  <Table.Td ta="right">{formatCents(row.aCents, locale)}</Table.Td>
                  <Table.Td ta="right">{formatCents(row.bCents, locale)}</Table.Td>
                  <Table.Td ta="right">
                    <Text size="sm" fw={600} c={row.diff === 0 ? 'dimmed' : row.diff > 0 ? 'red' : 'green'} span>
                      {row.diff >= 0 ? '+' : ''}{formatCents(row.diff, locale)}
                    </Text>
                  </Table.Td>
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
    </Stack>
  )
}
