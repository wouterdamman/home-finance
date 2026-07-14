import { useState, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Tabs, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, TextInput, Button,
} from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useMonthOverview } from '../api/hooks/usePeriods'
import { useTransactions, useCreateTransaction, useDeleteTransaction } from '../api/hooks/useTransactions'
import MoneyText from '../components/MoneyText'
import { parseToCents } from '../lib/money'

function parseCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

export default function MonthTransactions() {
  const { year, month } = useParams<{ year: string; month: string }>()
  const { t } = useTranslation()
  const y = Number(year)
  const m = Number(month)

  const summary = useYearSummary(y)
  const monthData = summary.data?.months?.find((mo) => mo.month === m)
  const periodId = monthData?.periodId

  const overviewQuery = useMonthOverview(periodId)
  const overview = overviewQuery.data

  if (summary.isLoading || overviewQuery.isLoading) return <Skeleton h={400} />
  if (!overview || !periodId) return (
    <Alert color="yellow">
      {t('month.notFound')} <Text component={Link} to={`/years/${y}`} c="blue">{t('common.back')}</Text>
    </Alert>
  )

  const isClosed = overview.period.status === 'closed'
  const itemizedLines = overview.budgetLines.filter(bl => bl.tracksTransactions)
  const defaultCatId = itemizedLines[0]?.categoryId ?? 0

  return (
    <Stack gap="md">
      <Group>
        <Text component={Link} to={`/months/${y}/${m}`} c="blue" size="sm">← {t('month.overviewLink')}</Text>
        <Title order={2}>{t('month.transactionsTitle', { month: m, year: y })}</Title>
      </Group>

      <Tabs defaultValue={String(defaultCatId)} keepMounted={false}>
        <Tabs.List>
          {itemizedLines.map(bl => (
            <Tabs.Tab key={bl.categoryId} value={String(bl.categoryId)}>
              {bl.label}
              <Text span size="xs" c="dimmed" ml="xs">
                (<MoneyText cents={bl.effectiveCents} />)
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
  const { data: txs, isLoading } = useTransactions(periodId, categoryId)
  const createTx = useCreateTransaction(periodId)
  const deleteTx = useDeleteTransaction(periodId)

  const [desc, setDesc] = useState('')
  const [amount, setAmount] = useState<number | string>('')
  const amountRef = useRef<HTMLInputElement>(null)

  const total = (txs ?? []).reduce((sum, tx) => sum + tx.amountCents, 0)

  const handleAdd = () => {
    const cents = parseCents(amount)
    if (!cents) return
    createTx.mutate({ categoryId, amountCents: cents, description: desc }, {
      onSuccess: () => { setDesc(''); setAmount('') }
    })
  }

  if (isLoading) return <Skeleton h={200} />

  return (
    <Stack gap="sm">
      {!isClosed && (
        <Group gap="xs" align="flex-end">
          <TextInput
            placeholder={t('common.description')}
            value={desc}
            onChange={e => setDesc(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') amountRef.current?.focus() }}
            flex={1}
          />
          <NumberInput
            ref={amountRef}
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

      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('common.description')}</Table.Th>
            <Table.Th ta="right">{t('common.amount')}</Table.Th>
            {!isClosed && <Table.Th w={40} />}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(txs ?? []).map(tx => (
            <Table.Tr key={tx.id}>
              <Table.Td>{tx.description}</Table.Td>
              <Table.Td ta="right"><MoneyText cents={tx.amountCents} /></Table.Td>
              {!isClosed && (
                <Table.Td>
                  <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => deleteTx.mutate(tx.id)}>✕</ActionIcon>
                </Table.Td>
              )}
            </Table.Tr>
          ))}
        </Table.Tbody>
        <Table.Tfoot>
          <Table.Tr fw={700}>
            <Table.Td>{t('common.total')}</Table.Td>
            <Table.Td ta="right"><MoneyText cents={total} /></Table.Td>
          </Table.Tr>
        </Table.Tfoot>
      </Table>
    </Stack>
  )
}
