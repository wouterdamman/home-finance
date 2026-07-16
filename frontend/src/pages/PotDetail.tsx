import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Badge, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, Paper, TextInput, Select, Progress,
} from '@mantine/core'
import { DateInput } from '@mantine/dates'
import { AreaChart } from '@mantine/charts'
import { modals } from '@mantine/modals'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { usePots, usePotLedger, useCreatePotEntry, useDeletePotEntry } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import { parseToCents } from '../lib/money'
import { getErrorMessage } from '../api/client'
import { notifications } from '@mantine/notifications'

const ENTRY_TYPE_COLORS: Record<string, string> = {
  allocation: 'blue',
  carryover_out: 'gray',
  withdrawal: 'red',
  deposit: 'green',
  adjustment: 'orange',
  opening_balance: 'gray',
}

const MANUAL_ENTRY_TYPES = ['deposit', 'withdrawal', 'adjustment', 'opening_balance']

function parseCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

export default function PotDetail() {
  const { id } = useParams<{ id: string }>()
  const potId = Number(id)
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  const { data: pots, isLoading: potsLoading } = usePots()
  const { data: ledger, isLoading: ledgerLoading } = usePotLedger(potId)
  const createEntry = useCreatePotEntry(potId)
  const deleteEntry = useDeletePotEntry(potId)

  const [entryType, setEntryType] = useState<string | null>('deposit')
  const [amount, setAmount] = useState<number | string>('')
  const [description, setDescription] = useState('')
  const [entryDate, setEntryDate] = useState<string | null>(dayjs().format('YYYY-MM-DD'))

  if (potsLoading || ledgerLoading) return <Skeleton h={400} mt="md" />

  const pot = pots?.find((p) => p.id === potId)
  if (!pot) return <Alert color="yellow">{t('pots.notFound')}</Alert>

  const entries = ledger ?? []
  const currentBalance = entries.length > 0 ? entries[entries.length - 1].runningBalance : 0
  const chartData = entries.map((e) => ({
    date: e.entryDate ? dayjs(e.entryDate).format('DD-MM') : '',
    [t('pots.runningBalance')]: e.runningBalance / 100,
  }))

  const handleAdd = () => {
    if (!entryType) return
    const cents = parseCents(amount)
    if (!cents) return
    createEntry.mutate(
      { entryType, amountCents: cents, description, entryDate: entryDate ?? undefined },
      {
        onSuccess: () => { setAmount(''); setDescription('') },
        onError: (err: unknown) => notifications.show({ color: 'red', message: getErrorMessage(err, t('common.error')) }),
      }
    )
  }

  const handleDelete = (entryId: number) => modals.openConfirmModal({
    title: t('common.delete'),
    children: <Text size="sm">{t('common.confirm')}</Text>,
    labels: { confirm: t('common.delete'), cancel: t('common.cancel') },
    confirmProps: { color: 'red' },
    onConfirm: () => deleteEntry.mutate(entryId),
  })

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Group gap="sm">
          <Text component={Link} to="/pots" c="blue" size="sm">{t('pots.back')}</Text>
          <Title order={2}>{pot.name}</Title>
          <Badge size="sm" variant="light" color={pot.kind === 'carryover' ? 'gray' : 'blue'}>
            {t(`pots.kind_${pot.kind}`)}
          </Badge>
        </Group>
        <MoneyText cents={currentBalance} size="xl" fw={800} />
      </Group>

      {pot.targetCents != null && pot.targetCents > 0 && (
        <Paper shadow="xs" p="md" withBorder>
          <Group justify="space-between" mb="xs">
            <Text size="sm" fw={600}>{t('pots.target')}</Text>
            <Group gap="xs">
              <MoneyText cents={pot.targetCents} size="sm" span />
              {pot.targetDate && <Text size="sm" c="dimmed">· {dayjs(pot.targetDate).format('DD-MM-YYYY')}</Text>}
            </Group>
          </Group>
          <Progress
            value={Math.min(100, Math.max(0, (currentBalance / pot.targetCents) * 100))}
            color={currentBalance >= pot.targetCents ? 'green' : 'blue'}
          />
        </Paper>
      )}

      {chartData.length > 1 && (
        <Paper shadow="xs" p="md" withBorder>
          <AreaChart
            h={220}
            data={chartData}
            dataKey="date"
            series={[{ name: t('pots.runningBalance'), color: 'blue.6' }]}
            curveType="linear"
            valueFormatter={(v) => new Intl.NumberFormat(locale, { style: 'currency', currency: 'EUR' }).format(v)}
          />
        </Paper>
      )}

      <Paper shadow="xs" p="md" withBorder>
        <Title order={4} mb="sm">{t('pots.ledger')}</Title>

        <Group gap="xs" align="flex-end" mb="md">
          <Select
            label={t('pots.type')}
            data={MANUAL_ENTRY_TYPES.map((v) => ({ value: v, label: t(`pots.entry_${v}`) }))}
            value={entryType}
            onChange={setEntryType}
            w={160}
          />
          <DateInput
            label={t('common.date')}
            value={entryDate}
            onChange={setEntryDate}
            valueFormat="DD-MM-YYYY"
            w={140}
          />
          <TextInput
            label={t('pots.description')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            flex={1}
          />
          <NumberInput
            label={t('common.amount')}
            value={amount}
            onChange={setAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
            placeholder="0,00"
            w={140}
          />
          <ActionIcon size="lg" variant="filled" aria-label={t('pots.book')} loading={createEntry.isPending} onClick={handleAdd}>+</ActionIcon>
        </Group>

        {entries.length === 0 ? (
          <Text c="dimmed">{t('pots.noEntries')}</Text>
        ) : (
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('pots.date')}</Table.Th>
                <Table.Th>{t('pots.type')}</Table.Th>
                <Table.Th>{t('pots.description')}</Table.Th>
                <Table.Th ta="right">{t('common.amount')}</Table.Th>
                <Table.Th ta="right">{t('pots.runningBalance')}</Table.Th>
                <Table.Th w={40} />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {[...entries].reverse().map((e) => (
                <Table.Tr key={e.id}>
                  <Table.Td>
                    <Text size="sm">{e.entryDate ? dayjs(e.entryDate).format('DD-MM-YYYY') : '—'}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Badge size="xs" variant="light" color={ENTRY_TYPE_COLORS[e.entryType] ?? 'gray'}>
                      {t(`pots.entry_${e.entryType}`)}
                    </Badge>
                  </Table.Td>
                  <Table.Td><Text size="sm">{e.description}</Text></Table.Td>
                  <Table.Td ta="right"><MoneyText cents={e.amountCents} colored /></Table.Td>
                  <Table.Td ta="right"><MoneyText cents={e.runningBalance} /></Table.Td>
                  <Table.Td>
                    {MANUAL_ENTRY_TYPES.includes(e.entryType) && (
                      <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => handleDelete(e.id)}>✕</ActionIcon>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Paper>
    </Stack>
  )
}
