import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Badge, Skeleton, Alert, Table, Button, Box,
  NumberInput, ActionIcon, Stack, Paper, TextInput, Select, Progress,
} from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { DateInput } from '@mantine/dates'
import { AreaChart } from '@mantine/charts'
import { modals } from '@mantine/modals'
import { IconTrash, IconPlus, IconPencil, IconCheck, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { usePots, usePotLedger, useCreatePotEntry, useUpdatePotEntry, useDeletePotEntry } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'
import { parseToCents } from '../lib/money'
import { getErrorMessage } from '../api/client'
import { notifications } from '@mantine/notifications'
import HeroStat from '../components/mobile/HeroStat'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import BottomSheet from '../components/mobile/BottomSheet'
import { useChartPalette } from '../contexts/ChartPaletteContext'

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
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  const { data: pots, isLoading: potsLoading } = usePots()
  const { data: ledger, isLoading: ledgerLoading } = usePotLedger(potId)
  const createEntry = useCreatePotEntry(potId)
  const updateEntry = useUpdatePotEntry(potId)
  const deleteEntry = useDeletePotEntry(potId)

  const [entryType, setEntryType] = useState<string | null>('deposit')
  const [amount, setAmount] = useState<number | string>('')
  const [description, setDescription] = useState('')
  const [entryDate, setEntryDate] = useState<string | null>(dayjs().format('YYYY-MM-DD'))
  const [addSheetOpen, setAddSheetOpen] = useState(false)
  const [editingEntryId, setEditingEntryId] = useState<number | null>(null)
  const [editAmount, setEditAmount] = useState<number | string>('')

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
        onSuccess: () => { setAmount(''); setDescription(''); setAddSheetOpen(false) },
        onError: (err: unknown) => notifications.show({ color: 'red', message: getErrorMessage(err, t('common.error')) }),
      }
    )
  }

  const startEditEntry = (entryId: number, amountCents: number) => {
    setEditingEntryId(entryId)
    setEditAmount(Math.abs(amountCents) / 100)
  }

  const saveEditEntry = () => {
    if (editingEntryId == null) return
    const cents = parseCents(editAmount)
    updateEntry.mutate(
      { id: editingEntryId, amountCents: cents },
      {
        onSuccess: () => setEditingEntryId(null),
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

  if (isMobile) {
    return (
      <Stack gap="md">
        <Group gap="sm">
          <Text component={Link} to="/pots" c="blue" size="sm">{t('pots.back')}</Text>
        </Group>

        <HeroStat label={pot.name} value={<MoneyText cents={currentBalance} span fw={800} size="2.5rem" />} />
        <Group justify="center">
          <Badge size="sm" variant="light" color={pot.kind === 'carryover' ? 'gray' : 'blue'}>
            {t(`pots.kind_${pot.kind}`)}
          </Badge>
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
          <Paper shadow="xs" p="sm" withBorder>
            <AreaChart
              h={160}
              data={chartData}
              dataKey="date"
              series={[{ name: t('pots.runningBalance'), color: palette.surplus }]}
              curveType="linear"
              valueFormatter={(v) => new Intl.NumberFormat(locale, { style: 'currency', currency: 'EUR' }).format(v)}
            />
          </Paper>
        )}

        <Group justify="space-between">
          <Text fw={600} size="sm">{t('pots.ledger')}</Text>
          <Button size="xs" leftSection={<IconPlus size={14} />} onClick={() => setAddSheetOpen(true)}>{t('pots.booking')}</Button>
        </Group>

        {entries.length === 0 ? (
          <EmptyState message={t('pots.noEntries')} />
        ) : (
          <MobileList>
            {[...entries].reverse().map((e) => (
              <MobileListRow
                key={e.id}
                leftSection={<Box w={10} h={10} bg={`${ENTRY_TYPE_COLORS[e.entryType] ?? 'gray'}.6`} style={{ borderRadius: '50%', flexShrink: 0 }} />}
                title={e.description || t(`pots.entry_${e.entryType}`)}
                subtitle={e.entryDate ? dayjs(e.entryDate).format('DD-MM-YYYY') : undefined}
                trailing={
                  <Stack gap={0} align="flex-end">
                    <MoneyText cents={e.amountCents} colored fw={600} size="sm" />
                    <MoneyText cents={e.runningBalance} c="dimmed" size="xs" />
                  </Stack>
                }
                chevron={MANUAL_ENTRY_TYPES.includes(e.entryType)}
                onClick={MANUAL_ENTRY_TYPES.includes(e.entryType) ? () => startEditEntry(e.id, e.amountCents) : undefined}
                swipeAction={MANUAL_ENTRY_TYPES.includes(e.entryType) ? {
                  label: <IconTrash size={18} />,
                  destructive: true,
                  onTrigger: () => deleteEntry.mutate(e.id),
                } : undefined}
              />
            ))}
          </MobileList>
        )}

        <BottomSheet opened={addSheetOpen} onClose={() => setAddSheetOpen(false)} title={t('pots.booking')}>
          <Select
            label={t('pots.type')}
            data={MANUAL_ENTRY_TYPES.map((v) => ({ value: v, label: t(`pots.entry_${v}`) }))}
            value={entryType}
            onChange={setEntryType}
          />
          <DateInput
            label={t('common.date')}
            value={entryDate}
            onChange={setEntryDate}
            valueFormat="DD-MM-YYYY"
          />
          <TextInput
            label={t('pots.description')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
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
          />
          <Button loading={createEntry.isPending} onClick={handleAdd}>{t('pots.book')}</Button>
        </BottomSheet>

        <BottomSheet opened={editingEntryId !== null} onClose={() => setEditingEntryId(null)} title={t('common.edit')}>
          <NumberInput
            label={t('common.amount')}
            value={editAmount}
            onChange={setEditAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
            placeholder="0,00"
          />
          <Button loading={updateEntry.isPending} onClick={saveEditEntry}>{t('common.save')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

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
            series={[{ name: t('pots.runningBalance'), color: palette.surplus }]}
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
          <EmptyState message={t('pots.noEntries')} />
        ) : (
          <Table.ScrollContainer minWidth={650}>
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
                  <Table.Td ta="right">
                    {editingEntryId === e.id ? (
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
                        <ActionIcon size="sm" color="green" variant="subtle" aria-label={t('common.save')} loading={updateEntry.isPending} onClick={saveEditEntry}><IconCheck size={14} /></ActionIcon>
                        <ActionIcon size="sm" variant="subtle" aria-label={t('common.cancel')} onClick={() => setEditingEntryId(null)}><IconX size={14} /></ActionIcon>
                      </Group>
                    ) : (
                      <MoneyText cents={e.amountCents} colored />
                    )}
                  </Table.Td>
                  <Table.Td ta="right"><MoneyText cents={e.runningBalance} /></Table.Td>
                  <Table.Td>
                    {MANUAL_ENTRY_TYPES.includes(e.entryType) && editingEntryId !== e.id && (
                      <Group gap={4} wrap="nowrap">
                        <ActionIcon size="sm" variant="subtle" aria-label={t('common.edit')} onClick={() => startEditEntry(e.id, e.amountCents)}><IconPencil size={14} /></ActionIcon>
                        <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => handleDelete(e.id)}><IconTrash size={14} /></ActionIcon>
                      </Group>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
          </Table.ScrollContainer>
        )}
      </Paper>
    </Stack>
  )
}
