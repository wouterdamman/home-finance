import { useState, useRef, useMemo } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Tabs, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, Button, Autocomplete,
} from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { modals } from '@mantine/modals'
import { DateInput } from '@mantine/dates'
import { IconTrash, IconPencil, IconCheck, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import EmptyState from '../components/EmptyState'
import dayjs from 'dayjs'
import { useYearSummary, useMonthOverview } from '../api/hooks/usePeriods'
import { useIncomeTransactions, useCreateIncomeTransaction, useUpdateIncomeTransaction, useDeleteIncomeTransaction } from '../api/hooks/useIncomeTransactions'
import { useIncomeSourceDescriptionPresets, useIncomeSourceTransactionDescriptions, mergeDescriptionSuggestions } from '../api/hooks/useDescriptionSuggestions'
import { useIncomeSources } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import { parseToCents } from '../lib/money'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import BottomSheet from '../components/mobile/BottomSheet'

function parseCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

export default function IncomeTransactions() {
  const { year, month } = useParams<{ year: string; month: string }>()
  const [searchParams] = useSearchParams()
  const { t } = useTranslation()
  const y = Number(year)
  const m = Number(month)

  const summary = useYearSummary(y)
  const monthData = summary.data?.months?.find((mo) => mo.month === m)
  const periodId = monthData?.periodId

  const overviewQuery = useMonthOverview(periodId)
  const overview = overviewQuery.data
  const { data: sources } = useIncomeSources()

  if (summary.isLoading || overviewQuery.isLoading) return <Skeleton h={400} />
  if (summary.error || overviewQuery.error) return <Alert color="red">{t('common.error')}</Alert>
  if (!overview || !periodId) return (
    <Alert color="yellow">
      {t('month.notFound')} <Text component={Link} to={`/years/${y}`} c="blue">{t('common.back')}</Text>
    </Alert>
  )

  const isClosed = overview.period.status === 'closed'
  // A label-only itemized entry has no source, so it can't key a tab or scope
  // the ?sourceId= query — only reachable via import, never via the UI.
  const itemizedIncomes = overview.incomes.filter(
    (inc): inc is typeof inc & { sourceId: number } => inc.isItemized && inc.sourceId != null,
  )
  const defaultSourceId = itemizedIncomes[0]?.sourceId ?? 0
  const sourceParam = searchParams.get('source')
  const initialSourceId = itemizedIncomes.some(inc => String(inc.sourceId) === sourceParam)
    ? sourceParam!
    : String(defaultSourceId)
  const sourceById = new Map((sources ?? []).map(s => [s.id, s.name] as const))
  const incomeLabel = (inc: typeof itemizedIncomes[number]) =>
    inc.label ?? (inc.sourceId != null ? sourceById.get(inc.sourceId) : undefined) ?? '—'

  return (
    <Stack gap="md">
      <Group>
        <Text component={Link} to={`/months/${y}/${m}`} c="blue" size="sm">← {t('month.overviewLink')}</Text>
        <Title order={2}>{t('month.incomeTransactionsTitle', { month: m, year: y })}</Title>
      </Group>

      {itemizedIncomes.length === 0 ? (
        <EmptyState message={t('month.noItemizedSources')} />
      ) : (
        <Tabs defaultValue={initialSourceId} keepMounted={false}>
          <Tabs.List>
            {itemizedIncomes.map(inc => (
              <Tabs.Tab key={inc.id} value={String(inc.sourceId)}>
                {incomeLabel(inc)}
                <Text span size="xs" c="dimmed" ml="xs">
                  (<MoneyText span cents={inc.effectiveCents} />)
                </Text>
              </Tabs.Tab>
            ))}
          </Tabs.List>

          {itemizedIncomes.map(inc => (
            <Tabs.Panel key={inc.id} value={String(inc.sourceId)} pt="md">
              <SourceTab
                periodId={periodId}
                sourceId={inc.sourceId}
                isClosed={isClosed}
              />
            </Tabs.Panel>
          ))}
        </Tabs>
      )}
    </Stack>
  )
}

function SourceTab({ periodId, sourceId, isClosed }: { periodId: number; sourceId: number; isClosed: boolean }) {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)', undefined, { getInitialValueInEffect: false })
  const { data: txs, isLoading } = useIncomeTransactions(periodId, sourceId)
  const { data: descPresets } = useIncomeSourceDescriptionPresets(sourceId)
  const { data: descHistory } = useIncomeSourceTransactionDescriptions(sourceId)
  const createTx = useCreateIncomeTransaction(periodId)
  const updateTx = useUpdateIncomeTransaction(periodId)
  const deleteTx = useDeleteIncomeTransaction(periodId)

  const [desc, setDesc] = useState('')
  const [amount, setAmount] = useState<number | string>('')
  const [txDate, setTxDate] = useState<string | null>(dayjs().format('YYYY-MM-DD'))
  const [dateSheetOpen, setDateSheetOpen] = useState(false)
  const amountRef = useRef<HTMLInputElement>(null)
  const [editingTxId, setEditingTxId] = useState<number | null>(null)
  const [editAmount, setEditAmount] = useState<number | string>('')

  const descriptionData = useMemo(() => mergeDescriptionSuggestions(descPresets, descHistory), [descPresets, descHistory])

  const total = (txs ?? []).reduce((sum, tx) => sum + tx.amountCents, 0)
  const sortedTxs = [...(txs ?? [])].sort((a, b) => (b.txDate ?? '').localeCompare(a.txDate ?? ''))

  const handleAdd = () => {
    const cents = parseCents(amount)
    if (!cents) return
    createTx.mutate({ sourceId, amountCents: cents, description: desc, txDate: txDate ?? undefined }, {
      onSuccess: () => { setDesc(''); setAmount('') }
    })
  }

  const startEditTx = (tx: { id: number; amountCents: number }) => {
    setEditingTxId(tx.id)
    setEditAmount(tx.amountCents / 100)
  }

  const saveEditTx = () => {
    const tx = (txs ?? []).find(t => t.id === editingTxId)
    if (!tx) return
    updateTx.mutate(
      { id: tx.id, amountCents: parseCents(editAmount), description: tx.description, txDate: tx.txDate ?? undefined },
      { onSuccess: () => setEditingTxId(null) }
    )
  }

  const confirmDeleteTx = (tx: { id: number; description: string }) => modals.openConfirmModal({
    title: t('common.delete'),
    children: <Text size="sm">{t('month.deleteTransactionConfirm', { description: tx.description || '—' })}</Text>,
    labels: { confirm: t('common.delete'), cancel: t('common.cancel') },
    confirmProps: { color: 'red' },
    onConfirm: () => deleteTx.mutate(tx.id),
  })

  if (isLoading) return <Skeleton h={200} />

  if (isMobile) {
    return (
      <Stack gap="sm">
        {!isClosed && (
          <Stack gap="xs">
            <Group gap="xs" wrap="nowrap">
              <Button variant="light" size="sm" onClick={() => setDateSheetOpen(true)}>
                {dayjs(txDate).format('DD-MM')}
              </Button>
              <Autocomplete
                placeholder={t('common.description')}
                value={desc}
                onChange={setDesc}
                data={descriptionData}
                onKeyDown={e => { if (e.key === 'Enter') amountRef.current?.focus() }}
                style={{ flex: 1 }}
              />
            </Group>
            <Group gap="xs" wrap="nowrap">
              <NumberInput
                ref={amountRef}
                placeholder="0,00"
                value={amount}
                onChange={setAmount}
                decimalSeparator=","
                decimalScale={2}
                prefix="€ "
                hideControls
                style={{ flex: 1 }}
                onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
              />
              <ActionIcon size="lg" variant="filled" aria-label={t('common.add')} loading={createTx.isPending} onClick={handleAdd}>+</ActionIcon>
            </Group>
          </Stack>
        )}

        {sortedTxs.length === 0 ? (
          <EmptyState message={t('month.noTransactions')} />
        ) : (
          <MobileList>
            {sortedTxs.map(tx => (
              <MobileListRow
                key={tx.id}
                title={tx.description || '—'}
                subtitle={tx.txDate ? dayjs(tx.txDate).format('DD-MM-YYYY') : undefined}
                trailing={<MoneyText cents={tx.amountCents} fw={600} />}
                chevron={!isClosed}
                onClick={!isClosed ? () => startEditTx(tx) : undefined}
                swipeAction={!isClosed ? {
                  label: <IconTrash size={18} />,
                  destructive: true,
                  onTrigger: () => deleteTx.mutate(tx.id),
                } : undefined}
              />
            ))}
          </MobileList>
        )}

        <Group justify="space-between" px="xs">
          <Text fw={700} size="sm">{t('common.total')}</Text>
          <MoneyText cents={total} fw={700} />
        </Group>

        <BottomSheet opened={dateSheetOpen} onClose={() => setDateSheetOpen(false)} title={t('common.date')}>
          <DateInput
            value={txDate}
            onChange={(v) => { setTxDate(v); setDateSheetOpen(false) }}
            valueFormat="DD-MM-YYYY"
          />
        </BottomSheet>

        <BottomSheet opened={editingTxId !== null} onClose={() => setEditingTxId(null)} title={t('common.edit')}>
          <NumberInput
            label={t('common.amount')}
            value={editAmount}
            onChange={setEditAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
          />
          <Button loading={updateTx.isPending} onClick={saveEditTx}>{t('common.save')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

  return (
    <Stack gap="sm">
      {!isClosed && (
        <Group gap="xs" align="flex-end">
          <DateInput
            value={txDate}
            onChange={setTxDate}
            valueFormat="DD-MM-YYYY"
            label={t('common.date')}
            w={140}
          />
          <Autocomplete
            label={t('common.description')}
            value={desc}
            onChange={setDesc}
            data={descriptionData}
            onKeyDown={e => { if (e.key === 'Enter') amountRef.current?.focus() }}
            flex={1}
          />
          <NumberInput
            ref={amountRef}
            label={t('common.amount')}
            placeholder="0,00"
            value={amount}
            onChange={setAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
            w={140}
            onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
          />
          <Button aria-label={t('common.add')} onClick={handleAdd} loading={createTx.isPending}>+</Button>
        </Group>
      )}

      <Table.ScrollContainer minWidth={420}>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('common.date')}</Table.Th>
            <Table.Th>{t('common.description')}</Table.Th>
            <Table.Th ta="right">{t('common.amount')}</Table.Th>
            {!isClosed && <Table.Th w={40} />}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {sortedTxs.length === 0 && (
            <Table.Tr>
              <Table.Td colSpan={isClosed ? 3 : 4}>
                <EmptyState message={t('month.noTransactions')} />
              </Table.Td>
            </Table.Tr>
          )}
          {sortedTxs.map(tx => (
            <Table.Tr key={tx.id}>
              <Table.Td>
                <Text size="sm" c={tx.txDate ? undefined : 'dimmed'}>
                  {tx.txDate ? dayjs(tx.txDate).format('DD-MM-YYYY') : '—'}
                </Text>
              </Table.Td>
              <Table.Td>{tx.description}</Table.Td>
              <Table.Td ta="right">
                {editingTxId === tx.id ? (
                  <Group justify="flex-end" wrap="nowrap" gap={4}>
                    <NumberInput
                      size="xs"
                      value={editAmount}
                      onChange={setEditAmount}
                      decimalSeparator=","
                      decimalScale={2}
                      prefix="€ "
                      hideControls
                      w={110}
                      autoFocus
                    />
                    <ActionIcon size="sm" color="green" variant="subtle" aria-label={t('common.save')} loading={updateTx.isPending} onClick={saveEditTx}><IconCheck size={14} /></ActionIcon>
                    <ActionIcon size="sm" variant="subtle" aria-label={t('common.cancel')} onClick={() => setEditingTxId(null)}><IconX size={14} /></ActionIcon>
                  </Group>
                ) : (
                  <MoneyText cents={tx.amountCents} />
                )}
              </Table.Td>
              {!isClosed && (
                <Table.Td>
                  {editingTxId !== tx.id && (
                    <Group gap={4} wrap="nowrap">
                      <ActionIcon size="sm" variant="subtle" aria-label={t('common.edit')} onClick={() => startEditTx(tx)}><IconPencil size={14} /></ActionIcon>
                      <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => confirmDeleteTx(tx)}><IconTrash size={14} /></ActionIcon>
                    </Group>
                  )}
                </Table.Td>
              )}
            </Table.Tr>
          ))}
        </Table.Tbody>
        <Table.Tfoot>
          <Table.Tr fw={700}>
            <Table.Td />
            <Table.Td>{t('common.total')}</Table.Td>
            <Table.Td ta="right"><MoneyText cents={total} /></Table.Td>
          </Table.Tr>
        </Table.Tfoot>
      </Table>
      </Table.ScrollContainer>
    </Stack>
  )
}
