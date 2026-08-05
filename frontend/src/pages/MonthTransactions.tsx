import { useState, useRef } from 'react'
import { useParams, useSearchParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Tabs, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, Button, Autocomplete,
} from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { DateInput } from '@mantine/dates'
import { IconTrash } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import EmptyState from '../components/EmptyState'
import dayjs from 'dayjs'
import { useYearSummary, useMonthOverview } from '../api/hooks/usePeriods'
import { useTransactions, useCreateTransaction, useDeleteTransaction } from '../api/hooks/useTransactions'
import { useCategoryDescriptionPresets } from '../api/hooks/useDescriptionSuggestions'
import { useCategories } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import { parseToCents } from '../lib/money'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import BottomSheet from '../components/mobile/BottomSheet'

function parseCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

export default function MonthTransactions() {
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
  const { data: categories } = useCategories()

  if (summary.isLoading || overviewQuery.isLoading) return <Skeleton h={400} />
  if (!overview || !periodId) return (
    <Alert color="yellow">
      {t('month.notFound')} <Text component={Link} to={`/years/${y}`} c="blue">{t('common.back')}</Text>
    </Alert>
  )

  const isClosed = overview.period.status === 'closed'
  const itemizedLines = overview.budgetLines.filter(bl => bl.tracksTransactions)
  const defaultCatId = itemizedLines[0]?.categoryId ?? 0
  const categoryParam = searchParams.get('category')
  const initialCatId = itemizedLines.some(bl => String(bl.categoryId) === categoryParam)
    ? categoryParam!
    : String(defaultCatId)
  const categoryById = new Map((categories ?? []).map(c => [c.id, c.name] as const))
  const budgetLineLabel = (bl: typeof itemizedLines[number]) =>
    bl.label ?? (bl.categoryId != null ? categoryById.get(bl.categoryId) : undefined) ?? '—'

  return (
    <Stack gap="md">
      <Group>
        <Text component={Link} to={`/months/${y}/${m}`} c="blue" size="sm">← {t('month.overviewLink')}</Text>
        <Title order={2}>{t('month.transactionsTitle', { month: m, year: y })}</Title>
      </Group>

      <Tabs defaultValue={initialCatId} keepMounted={false}>
        <Tabs.List>
          {itemizedLines.map(bl => (
            <Tabs.Tab key={bl.categoryId} value={String(bl.categoryId)}>
              {budgetLineLabel(bl)}
              <Text span size="xs" c="dimmed" ml="xs">
                (<MoneyText span cents={bl.effectiveCents} />)
              </Text>
            </Tabs.Tab>
          ))}
        </Tabs.List>

        {itemizedLines.map(bl => (
          <Tabs.Panel key={bl.categoryId} value={String(bl.categoryId)} pt="md">
            <CategoryTab
              periodId={periodId}
              categoryId={bl.categoryId ?? 0}
              isClosed={isClosed}
            />
          </Tabs.Panel>
        ))}
      </Tabs>
    </Stack>
  )
}

function CategoryTab({ periodId, categoryId, isClosed }: { periodId: number; categoryId: number; isClosed: boolean }) {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data: txs, isLoading } = useTransactions(periodId, categoryId)
  const { data: descPresets } = useCategoryDescriptionPresets(categoryId)
  const createTx = useCreateTransaction(periodId)
  const deleteTx = useDeleteTransaction(periodId)

  const [desc, setDesc] = useState('')
  const [amount, setAmount] = useState<number | string>('')
  const [txDate, setTxDate] = useState<string | null>(dayjs().format('YYYY-MM-DD'))
  const [dateSheetOpen, setDateSheetOpen] = useState(false)
  const amountRef = useRef<HTMLInputElement>(null)

  const descriptionData = (descPresets ?? []).map(p => p.description)

  const total = (txs ?? []).reduce((sum, tx) => sum + tx.amountCents, 0)
  const sortedTxs = [...(txs ?? [])].sort((a, b) => (b.txDate ?? '').localeCompare(a.txDate ?? ''))

  const handleAdd = () => {
    const cents = parseCents(amount)
    if (!cents) return
    createTx.mutate({ categoryId, amountCents: cents, description: desc, txDate: txDate ?? undefined }, {
      onSuccess: () => { setDesc(''); setAmount('') }
    })
  }

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
              <Table.Td ta="right"><MoneyText cents={tx.amountCents} /></Table.Td>
              {!isClosed && (
                <Table.Td>
                  <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => deleteTx.mutate(tx.id)}><IconTrash size={14} /></ActionIcon>
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
