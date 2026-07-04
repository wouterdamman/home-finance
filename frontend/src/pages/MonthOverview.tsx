import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Button, Badge, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, Paper, TextInput,
} from '@mantine/core'
import { modals } from '@mantine/modals'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useMonthOverview, useClosePeriod, useReopenPeriod, useUpdateBudgetLine } from '../api/hooks/usePeriods'
import { useUpdateIncome, useDeleteIncome, useCreateIncome } from '../api/hooks/useIncomes'
import { useReplaceSplits } from '../api/hooks/useSplits'
import MoneyText from '../components/MoneyText'

const MONTH_NL = ['','Januari','Februari','Maart','April','Mei','Juni','Juli','Augustus','September','Oktober','November','December']
const MONTH_EN = ['','January','February','March','April','May','June','July','August','September','October','November','December']

function parseCents(v: number | string): number {
  const n = typeof v === 'number' ? v : parseFloat(String(v).replace(',', '.'))
  return Math.round((n || 0) * 100)
}

export default function MonthOverview() {
  const { year, month } = useParams<{ year: string; month: string }>()
  const { t, i18n } = useTranslation()
  const y = Number(year)
  const m = Number(month)
  const monthNames = i18n.language.startsWith('nl') ? MONTH_NL : MONTH_EN

  const summary = useYearSummary(y)
  const monthData = summary.data?.months?.find((mo) => mo.month === m)
  const periodId = monthData?.periodId

  const overviewQuery = useMonthOverview(periodId)
  const overview = overviewQuery.data

  const [splitEdits, setSplitEdits] = useState<Record<number, string> | null>(null)
  const [newLabel, setNewLabel] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')

  const closePeriod = useClosePeriod(periodId ?? 0)
  const reopenPeriod = useReopenPeriod(periodId ?? 0)
  const updateBudgetLine = useUpdateBudgetLine(periodId ?? 0)
  const createIncome = useCreateIncome(periodId ?? 0)
  const updateIncome = useUpdateIncome(periodId ?? 0)
  const deleteIncome = useDeleteIncome(periodId ?? 0)
  const replaceSplits = useReplaceSplits(periodId ?? 0)

  if (summary.isLoading || overviewQuery.isLoading) return <Skeleton h={600} mt="md" />
  if (summary.error || overviewQuery.error) return <Alert color="red">{t('common.error')}</Alert>
  if (!periodId) return (
    <Alert color="yellow">
      {t('month.notCreated')} <Text component={Link} to={`/years/${y}`} c="blue">{t('month.backToYear')}</Text>
    </Alert>
  )
  if (!overview) return <Skeleton h={600} />

  const { period, incomes, incomeTotalCents, budgetLines, expenseTotalCents, surplusCents, splits } = overview
  const isClosed = period.status === 'closed'
  const monthName = monthNames[m]

  const handleClose = () => modals.openConfirmModal({
    title: `${monthName} ${y} ${t('month.closeAction').toLowerCase()}`,
    children: <Text size="sm">{t('month.closingConfirm')}</Text>,
    labels: { confirm: t('month.closeAction'), cancel: t('common.cancel') },
    confirmProps: { color: 'green' },
    onConfirm: () => closePeriod.mutate(undefined),
  })

  const handleReopen = () => modals.openConfirmModal({
    title: t('month.reopenMonth'),
    children: <Text size="sm">{t('month.reopenConfirm')}</Text>,
    labels: { confirm: t('month.reopenAction'), cancel: t('common.cancel') },
    confirmProps: { color: 'orange' },
    onConfirm: () => reopenPeriod.mutate(undefined),
  })

  const currentSplits = splitEdits ?? Object.fromEntries(splits.map(s => [s.potId, s.percentage]))

  const handleSaveSplits = () => {
    replaceSplits.mutate(
      Object.entries(currentSplits).map(([potId, pct]) => ({ potId: Number(potId), percentage: pct })),
      { onSuccess: () => setSplitEdits(null) }
    )
  }

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Group gap="sm">
          <Text component={Link} to={`/years/${y}`} c="blue" size="sm">← {y}</Text>
          <Title order={2}>{monthName} {y}</Title>
          <Badge color={isClosed ? 'green' : 'orange'}>
            {isClosed ? t('month.statusClosed') : t('month.statusOpen')}
          </Badge>
        </Group>
        <Group>
          <Button component={Link} to={`/months/${y}/${m}/transactions`} variant="subtle" size="sm">
            {t('month.transactions')}
          </Button>
          {isClosed
            ? <Button color="orange" onClick={handleReopen} loading={reopenPeriod.isPending}>{t('month.reopenAction')}</Button>
            : <Button color="green" onClick={handleClose} loading={closePeriod.isPending}>{t('month.closeAction')}</Button>
          }
        </Group>
      </Group>

      {/* Incomes */}
      <Paper shadow="xs" p="md" withBorder>
        <Title order={4} mb="sm">{t('month.income')}</Title>
        <Table>
          <Table.Tbody>
            {incomes.map(inc => (
              <Table.Tr key={inc.id}>
                <Table.Td>
                  <Text size="sm" c={inc.entryType === 'carryover' ? 'dimmed' : undefined}>
                    {inc.label ?? `Bron #${inc.sourceId}`}
                    {inc.entryType === 'carryover' && <Badge size="xs" ml="xs" color="gray">{t('month.carryoverBadge')}</Badge>}
                  </Text>
                </Table.Td>
                <Table.Td ta="right" w={160}>
                  {isClosed || inc.entryType === 'carryover'
                    ? <MoneyText cents={inc.amountCents} />
                    : <NumberInput
                        size="xs"
                        defaultValue={inc.amountCents / 100}
                        decimalSeparator=","
                        fixedDecimalScale
                        decimalScale={2}
                        prefix="€ "
                        hideControls
                        onBlur={(e) => {
                          const raw = e.target.value.replace('€ ', '').replace(/\./g, '').replace(',', '.')
                          const newCents = Math.round(parseFloat(raw) * 100) || 0
                          if (newCents !== inc.amountCents)
                            updateIncome.mutate({ id: inc.id, amountCents: newCents, notes: inc.notes, sortOrder: inc.sortOrder, label: inc.label })
                        }}
                      />
                  }
                </Table.Td>
                {!isClosed && inc.entryType !== 'carryover' && (
                  <Table.Td w={40}>
                    <ActionIcon color="red" size="sm" variant="subtle" onClick={() => deleteIncome.mutate(inc.id)}>✕</ActionIcon>
                  </Table.Td>
                )}
              </Table.Tr>
            ))}
          </Table.Tbody>
          <Table.Tfoot>
            {!isClosed && (
              <Table.Tr>
                <Table.Td>
                  <TextInput size="xs" placeholder={t('common.description')} value={newLabel} onChange={e => setNewLabel(e.target.value)} />
                </Table.Td>
                <Table.Td>
                  <NumberInput size="xs" value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls placeholder="0,00" />
                </Table.Td>
                <Table.Td>
                  <Button size="xs" disabled={!newLabel} loading={createIncome.isPending} onClick={() => {
                    createIncome.mutate({ label: newLabel, amountCents: parseCents(newAmount), notes: '', sortOrder: incomes.length }, {
                      onSuccess: () => { setNewLabel(''); setNewAmount('') }
                    })
                  }}>+</Button>
                </Table.Td>
              </Table.Tr>
            )}
            <Table.Tr fw={700}>
              <Table.Td>{t('month.totalIncome')}</Table.Td>
              <Table.Td ta="right"><MoneyText cents={incomeTotalCents} /></Table.Td>
            </Table.Tr>
          </Table.Tfoot>
        </Table>
      </Paper>

      {/* Budget lines */}
      <Paper shadow="xs" p="md" withBorder>
        <Title order={4} mb="sm">{t('month.expenses')}</Title>
        <Table>
          <Table.Tbody>
            {budgetLines.map(bl => (
              <Table.Tr key={bl.id}>
                <Table.Td>
                  {bl.tracksTransactions
                    ? <Text component={Link} to={`/months/${y}/${m}/transactions`} c="blue" size="sm">{bl.label}</Text>
                    : <Text size="sm">{bl.label}</Text>
                  }
                </Table.Td>
                <Table.Td ta="right">
                  <MoneyText cents={bl.effectiveCents} />
                  {bl.tracksTransactions && bl.amountCents > 0 && (
                    <Text size="xs" c="dimmed" span> / {(bl.amountCents / 100).toFixed(2)}</Text>
                  )}
                </Table.Td>
                {!isClosed && (
                  <Table.Td w={32}>
                    <ActionIcon
                      size="xs"
                      variant={bl.tracksTransactions ? 'filled' : 'subtle'}
                      color={bl.tracksTransactions ? 'blue' : 'gray'}
                      title={t('month.toggleTracking')}
                      onClick={() => updateBudgetLine.mutate({
                        id: bl.id,
                        label: bl.label ?? '',
                        amountCents: bl.amountCents,
                        tracksTransactions: !bl.tracksTransactions,
                        sortOrder: bl.sortOrder,
                      })}
                    >≡</ActionIcon>
                  </Table.Td>
                )}
              </Table.Tr>
            ))}
          </Table.Tbody>
          <Table.Tfoot>
            <Table.Tr fw={700}>
              <Table.Td>{t('month.totalExpenses')}</Table.Td>
              <Table.Td ta="right"><MoneyText cents={expenseTotalCents} /></Table.Td>
            </Table.Tr>
          </Table.Tfoot>
        </Table>
      </Paper>

      {/* Surplus banner */}
      <Paper shadow="xs" p="md" withBorder style={{ background: surplusCents >= 0 ? 'var(--mantine-color-green-0)' : 'var(--mantine-color-red-0)' }}>
        <Group justify="space-between">
          <Title order={3}>{t('month.surplusLabel')}</Title>
          <MoneyText cents={surplusCents} size="xl" fw={800} colored />
        </Group>
      </Paper>

      {/* Splits */}
      <Paper shadow="xs" p="md" withBorder>
        <Group justify="space-between" mb="sm">
          <Title order={4}>{t('month.splitSection')}</Title>
          {!isClosed && (
            splitEdits
              ? <Group gap="xs">
                  <Button size="xs" onClick={handleSaveSplits} loading={replaceSplits.isPending}>{t('common.save')}</Button>
                  <Button size="xs" variant="subtle" onClick={() => setSplitEdits(null)}>{t('common.cancel')}</Button>
                </Group>
              : <Button size="xs" variant="subtle" onClick={() => setSplitEdits(Object.fromEntries(splits.map(s => [s.potId, s.percentage])))}>
                  {t('month.adjust')}
                </Button>
          )}
        </Group>
        <Table>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('month.pot')}</Table.Th>
              <Table.Th ta="right">{t('month.percentage')}</Table.Th>
              <Table.Th ta="right">{t('month.amount')}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {splits.map(sp => (
              <Table.Tr key={sp.potId}>
                <Table.Td>
                  <Text component={Link} to={`/pots/${sp.potId}`} c="blue" size="sm">{sp.potName}</Text>
                </Table.Td>
                <Table.Td ta="right" w={120}>
                  {splitEdits
                    ? <NumberInput
                        size="xs"
                        value={Number(splitEdits[sp.potId] ?? sp.percentage)}
                        onChange={(v) => setSplitEdits(prev => ({ ...prev!, [sp.potId]: String(v) }))}
                        suffix="%"
                        decimalScale={2}
                        hideControls
                        min={0}
                        max={100}
                      />
                    : <Text size="sm">{sp.percentage}%</Text>
                  }
                </Table.Td>
                <Table.Td ta="right">
                  <MoneyText cents={sp.projectedCents ?? 0} colored={!isClosed} />
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Paper>
    </Stack>
  )
}
