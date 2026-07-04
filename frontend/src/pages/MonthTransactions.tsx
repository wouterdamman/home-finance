import { useState, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Tabs, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, TextInput, Button,
} from '@mantine/core'
import { useYearSummary, useMonthOverview } from '../api/hooks/usePeriods'
import { useTransactions, useCreateTransaction, useDeleteTransaction } from '../api/hooks/useTransactions'
import MoneyText from '../components/MoneyText'

function parseCents(v: number | string): number {
  const n = typeof v === 'number' ? v : parseFloat(String(v).replace(',', '.'))
  return Math.round((n || 0) * 100)
}

export default function MonthTransactions() {
  const { year, month } = useParams<{ year: string; month: string }>()
  const y = Number(year)
  const m = Number(month)

  const summary = useYearSummary(y)
  const monthData = summary.data?.months?.find((mo) => mo.month === m)
  const periodId = monthData?.periodId

  const overviewQuery = useMonthOverview(periodId)
  const overview = overviewQuery.data

  if (summary.isLoading || overviewQuery.isLoading) return <Skeleton h={400} />
  if (!overview || !periodId) return <Alert color="yellow">Maand niet gevonden. <Link to={`/years/${y}`}>Terug</Link></Alert>

  const isClosed = overview.period.status === 'closed'
  const itemizedLines = overview.budgetLines.filter(bl => bl.tracksTransactions)
  const defaultCatId = itemizedLines[0]?.categoryId ?? 0

  return (
    <Stack gap="md">
      <Group>
        <Text component={Link} to={`/months/${y}/${m}`} c="blue" size="sm">← Maandoverzicht</Text>
        <Title order={2}>Transacties — {m}/{y}</Title>
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
  const { data: txs, isLoading } = useTransactions(periodId, categoryId)
  const createTx = useCreateTransaction(periodId)
  const deleteTx = useDeleteTransaction(periodId)

  const [desc, setDesc] = useState('')
  const [amount, setAmount] = useState<number | string>('')
  const amountRef = useRef<HTMLInputElement>(null)

  const total = (txs ?? []).reduce((sum, t) => sum + t.amountCents, 0)

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
            placeholder="Omschrijving"
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
          <Button onClick={handleAdd} loading={createTx.isPending}>+</Button>
        </Group>
      )}

      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Omschrijving</Table.Th>
            <Table.Th ta="right">Bedrag</Table.Th>
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
                  <ActionIcon color="red" size="sm" variant="subtle" onClick={() => deleteTx.mutate(tx.id)}>✕</ActionIcon>
                </Table.Td>
              )}
            </Table.Tr>
          ))}
        </Table.Tbody>
        <Table.Tfoot>
          <Table.Tr fw={700}>
            <Table.Td>Totaal</Table.Td>
            <Table.Td ta="right"><MoneyText cents={total} /></Table.Td>
          </Table.Tr>
        </Table.Tfoot>
      </Table>
    </Stack>
  )
}
