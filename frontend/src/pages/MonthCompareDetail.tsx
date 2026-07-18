import { useMemo, useState } from 'react'
import { Title, Skeleton, Alert, Text, Stack, Group, Select, Paper, Table, Divider } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { useTrendsCategoryTotals, useYearSummary } from '../api/hooks/usePeriods'
import { formatCents } from '../lib/money'
import { MobileListRow } from '../components/mobile/MobileList'
import EmptyState from '../components/EmptyState'

const MONTHS_NL = ['jan', 'feb', 'mrt', 'apr', 'mei', 'jun', 'jul', 'aug', 'sep', 'okt', 'nov', 'dec']
const MONTHS_EN = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

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

  const categoryRows = categoryQuery.data.categories
    .map((cat) => {
      const aCents = entryA?.values[String(cat.id)] ?? 0
      const bCents = entryB?.values[String(cat.id)] ?? 0
      return { id: cat.id, name: cat.name, aCents, bCents, diff: bCents - aCents }
    })
    .sort((a, b) => Math.abs(b.diff) - Math.abs(a.diff))

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

      {isMobile ? (
        <Stack gap={0}>
          {categoryRows.map((row) => (
            <MobileListRow
              key={row.id}
              title={row.name}
              subtitle={`${formatCents(row.aCents, locale)} → ${formatCents(row.bCents, locale)}`}
              trailing={
                <Text size="sm" fw={600} c={row.diff === 0 ? 'dimmed' : row.diff > 0 ? 'red' : 'green'}>
                  {row.diff >= 0 ? '+' : ''}{formatCents(row.diff, locale)}
                </Text>
              }
            />
          ))}
        </Stack>
      ) : (
        <Table.ScrollContainer minWidth={480}>
          <Table striped>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('settings.categories')}</Table.Th>
                <Table.Th ta="right">{periodOptions.find((o) => o.value === effectiveAKey)?.label}</Table.Th>
                <Table.Th ta="right">{periodOptions.find((o) => o.value === effectiveBKey)?.label}</Table.Th>
                <Table.Th ta="right">Δ</Table.Th>
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
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
    </Stack>
  )
}
