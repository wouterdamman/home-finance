import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Button, Skeleton, Alert, Table,
  NumberInput, Select, Stack, Paper, TextInput, Modal,
} from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { usePotBalances, usePotLedger, useAddPotEntry } from '../api/hooks/usePots'
import MoneyText from '../components/MoneyText'

export default function PotDetail() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const potId = Number(id)
  const [opened, setOpened] = useState(false)
  const [entryType, setEntryType] = useState<string>('withdrawal')
  const [amount, setAmount] = useState<number | string>('')
  const [desc, setDesc] = useState('')

  const { data: balances, isLoading: balLoading } = usePotBalances()
  const { data: ledger, isLoading: ledLoading } = usePotLedger(potId)
  const addEntry = useAddPotEntry(potId)

  const pot = balances?.find(b => b.potId === potId)
  const today = new Date().toISOString().slice(0, 10)

  const handleSubmit = () => {
    const cents = Math.round((Number(String(amount).replace(',', '.')) || 0) * 100)
    if (!cents) return
    addEntry.mutate({ entryType, amountCents: cents, description: desc, entryDate: today }, {
      onSuccess: () => { setOpened(false); setAmount(''); setDesc('') }
    })
  }

  if (balLoading || ledLoading) return <Skeleton h={400} />
  if (!pot) return <Alert color="yellow">{t('pots.notFound')} <Link to="/pots">{t('pots.back')}</Link></Alert>

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Group gap="sm">
          <Text component={Link} to="/pots" c="blue" size="sm">{t('pots.back')}</Text>
          <Title order={2}>{pot.name}</Title>
        </Group>
        <Group>
          <Paper p="sm" shadow="xs" withBorder>
            <Text size="xs" c="dimmed">{t('pots.balance')}</Text>
            <MoneyText cents={pot.balanceCents} size="xl" fw={700} colored />
          </Paper>
          <Button onClick={() => setOpened(true)}>{t('pots.booking')}</Button>
        </Group>
      </Group>

      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('pots.date')}</Table.Th>
            <Table.Th>{t('pots.type')}</Table.Th>
            <Table.Th>{t('pots.description')}</Table.Th>
            <Table.Th ta="right">{t('common.amount')}</Table.Th>
            <Table.Th ta="right">{t('pots.runningBalance')}</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(ledger ?? []).map(entry => (
            <Table.Tr key={entry.id}>
              <Table.Td><Text size="sm" c="dimmed">{entry.entryDate ?? '-'}</Text></Table.Td>
              <Table.Td><Text size="sm">{t(`pots.entry_${entry.entryType}`, { defaultValue: entry.entryType })}</Text></Table.Td>
              <Table.Td><Text size="sm">{entry.description}</Text></Table.Td>
              <Table.Td ta="right"><MoneyText cents={entry.amountCents} colored /></Table.Td>
              <Table.Td ta="right"><MoneyText cents={entry.runningBalance} /></Table.Td>
            </Table.Tr>
          ))}
          {(ledger ?? []).length === 0 && (
            <Table.Tr>
              <Table.Td colSpan={5} ta="center"><Text c="dimmed" size="sm">{t('pots.noEntries')}</Text></Table.Td>
            </Table.Tr>
          )}
        </Table.Tbody>
      </Table>

      <Modal opened={opened} onClose={() => setOpened(false)} title={t('pots.manualEntry')}>
        <Stack gap="md">
          <Select
            label={t('pots.type')}
            value={entryType}
            onChange={v => setEntryType(v ?? 'withdrawal')}
            data={[
              { value: 'withdrawal', label: t('pots.withdraw') },
              { value: 'deposit', label: t('pots.deposit') },
              { value: 'adjustment', label: t('pots.entryAdjustment') },
              { value: 'opening_balance', label: t('pots.entryOpeningBalance') },
            ]}
          />
          <TextInput label={t('pots.description')} value={desc} onChange={e => setDesc(e.target.value)} />
          <NumberInput
            label={t('common.amount')}
            value={amount}
            onChange={setAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
          />
          <Button onClick={handleSubmit} loading={addEntry.isPending}>{t('pots.book')}</Button>
        </Stack>
      </Modal>
    </Stack>
  )
}
