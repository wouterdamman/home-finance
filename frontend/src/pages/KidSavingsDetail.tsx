import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  Title, Text, Group, Badge, Skeleton, Alert, Table, Button, Box,
  NumberInput, ActionIcon, Stack, Paper, TextInput, Select, SegmentedControl,
} from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { DateInput } from '@mantine/dates'
import { AreaChart } from '@mantine/charts'
import { modals } from '@mantine/modals'
import { IconTrash, IconPlus, IconPencil, IconCheck, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import {
  useKidBalances, useKidLedger, useCreateKidLedgerEntry,
  useUpdateKidLedgerEntry, useDeleteKidLedgerEntry, useUpdateKidReportedBalance,
} from '../api/hooks/useSettings'
import { useMe } from '../api/hooks/useMe'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'
import { parseToCents, formatCents, formatCentsCompact } from '../lib/money'
import { niceAxisTicksSigned } from '../lib/chartAxis'
import { getErrorMessage } from '../api/client'
import { notifications } from '@mantine/notifications'
import HeroStat from '../components/mobile/HeroStat'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import BottomSheet from '../components/mobile/BottomSheet'
import { useChartPalette } from '../contexts/ChartPaletteContext'

const ENTRY_TYPE_COLORS: Record<string, string> = {
  withdrawal: 'red',
  deposit: 'green',
  adjustment: 'orange',
  opening_balance: 'gray',
}

const MANUAL_ENTRY_TYPES = ['deposit', 'withdrawal', 'adjustment', 'opening_balance']

function parseCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

export default function KidSavingsDetail() {
  const { id } = useParams<{ id: string }>()
  const kidId = Number(id)
  const { t, i18n } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()

  const { data: balances, isLoading: balancesLoading } = useKidBalances()
  const { data: ledger, isLoading: ledgerLoading } = useKidLedger(kidId)
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'
  const createEntry = useCreateKidLedgerEntry(kidId)
  const updateEntry = useUpdateKidLedgerEntry(kidId)
  const deleteEntry = useDeleteKidLedgerEntry(kidId)
  const updateReported = useUpdateKidReportedBalance()

  const [owner, setOwner] = useState<'ours' | 'theirs'>('ours')
  const [entryType, setEntryType] = useState<string | null>('deposit')
  const [amount, setAmount] = useState<number | string>('')
  const [description, setDescription] = useState('')
  const [entryDate, setEntryDate] = useState<string | null>(dayjs().format('YYYY-MM-DD'))
  const [addSheetOpen, setAddSheetOpen] = useState(false)
  const [editingEntryId, setEditingEntryId] = useState<number | null>(null)
  const [editOwner, setEditOwner] = useState<'ours' | 'theirs'>('ours')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [reportedSheetOpen, setReportedSheetOpen] = useState(false)
  const [reportedAmount, setReportedAmount] = useState<number | string>('')
  const [reportedDate, setReportedDate] = useState<string | null>(dayjs().format('YYYY-MM-DD'))

  if (balancesLoading || ledgerLoading) return <Skeleton h={400} mt="md" />

  const balance = balances?.find((b) => b.kidId === kidId)
  if (!balance) return <Alert color="yellow">{t('kids.notFound')}</Alert>

  const entries = ledger ?? []
  const diff = balance.reportedBalanceCents != null ? balance.reportedBalanceCents - balance.totalCents : null
  const chartData = entries.map((e) => ({
    date: e.entryDate ? dayjs(e.entryDate).format('DD-MM') : '',
    [t('kids.ours')]: e.runningOursCents / 100,
    [t('kids.theirs')]: e.runningTheirsCents / 100,
  }))

  const handleAdd = () => {
    if (!entryType) return
    const cents = parseCents(amount)
    if (!cents) {
      notifications.show({ color: 'red', message: t('kids.amountRequired') })
      return
    }
    createEntry.mutate(
      { owner, entryType, amountCents: cents, description, entryDate: entryDate ?? undefined },
      {
        onSuccess: () => { setAmount(''); setDescription(''); setAddSheetOpen(false) },
        onError: (err: unknown) => notifications.show({ color: 'red', message: getErrorMessage(err, t('common.error')) }),
      }
    )
  }

  // The server re-applies the negative sign only for withdrawals; adjustment /
  // opening_balance are stored exactly as sent, so stripping the sign here
  // would silently flip a negative entry on an otherwise unchanged save.
  const startEditEntry = (entryId: number, entryOwner: 'ours' | 'theirs', entryType: string, amountCents: number) => {
    setEditingEntryId(entryId)
    setEditOwner(entryOwner)
    setEditAmount((entryType === 'withdrawal' ? Math.abs(amountCents) : amountCents) / 100)
  }

  const saveEditEntry = () => {
    if (editingEntryId == null) return
    const cents = parseCents(editAmount)
    updateEntry.mutate(
      { id: editingEntryId, owner: editOwner, amountCents: cents },
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

  const openReportedSheet = () => {
    setReportedAmount(balance.reportedBalanceCents != null ? balance.reportedBalanceCents / 100 : '')
    setReportedDate(balance.reportedBalanceDate ?? dayjs().format('YYYY-MM-DD'))
    setReportedSheetOpen(true)
  }

  const saveReported = () => {
    const cents = reportedAmount === '' ? null : parseCents(reportedAmount)
    updateReported.mutate(
      { id: kidId, reportedBalanceCents: cents, reportedBalanceDate: cents == null ? null : reportedDate },
      {
        onSuccess: () => setReportedSheetOpen(false),
        onError: (err: unknown) => notifications.show({ color: 'red', message: getErrorMessage(err, t('common.error')) }),
      }
    )
  }

  const ownerControl = (
    <SegmentedControl
      value={owner}
      onChange={(v) => setOwner(v as 'ours' | 'theirs')}
      data={[
        { label: t('kids.ours'), value: 'ours' },
        { label: t('kids.theirs'), value: 'theirs' },
      ]}
    />
  )

  const chartValues = chartData.flatMap((d) => [Number(d[t('kids.ours')]) || 0, Number(d[t('kids.theirs')]) || 0])
  // Stacked areas: the visible top of the chart is the sum of both series.
  const chartStackedMax = chartData.reduce((max, d) => Math.max(max, (Number(d[t('kids.ours')]) || 0) + (Number(d[t('kids.theirs')]) || 0)), 0)
  const chartTicks = niceAxisTicksSigned(Math.min(...chartValues, 0), Math.max(chartStackedMax, 0))
  const chartYAxisProps = {
    tickFormatter: (v: number) => formatCentsCompact(Math.round(v * 100), locale),
    width: 56,
    ticks: chartTicks,
    domain: [chartTicks[0], chartTicks[chartTicks.length - 1]] as [number, number],
  }
  const chartXAxisProps = { interval: Math.max(0, Math.ceil(chartData.length / 6) - 1) }

  const chartBlock = chartData.length > 1 && (
    <Paper shadow="xs" p={isMobile ? 'sm' : 'md'} withBorder>
      <AreaChart
        h={isMobile ? 160 : 220}
        data={chartData}
        dataKey="date"
        type="stacked"
        withLegend
        series={[
          { name: t('kids.ours'), color: palette.categorical[0] },
          { name: t('kids.theirs'), color: palette.categorical[1] },
        ]}
        curveType="linear"
        valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
        xAxisProps={chartXAxisProps}
        yAxisProps={chartYAxisProps}
      />
    </Paper>
  )

  const reportedBlock = balance.reportedBalanceCents != null && (
    <Paper shadow="xs" p="md" withBorder>
      <Group justify="space-between">
        <div>
          <Text size="sm" fw={600}>{t('kids.reportedBalance')}</Text>
          <Text size="xs" c="dimmed">
            {t('kids.reportedAsOf', { date: balance.reportedBalanceDate ? dayjs(balance.reportedBalanceDate).format('DD-MM-YYYY') : '' })}
          </Text>
        </div>
        <Group gap="xs">
          <MoneyText cents={balance.reportedBalanceCents} fw={700} />
          {diff !== 0 ? (
            <Badge color="orange" variant="light">{t('kids.diff')} <MoneyText cents={diff!} size="xs" span /></Badge>
          ) : (
            <Badge color="green" variant="light">{t('kids.matches')}</Badge>
          )}
        </Group>
      </Group>
    </Paper>
  )

  if (isMobile) {
    return (
      <Stack gap="md">
        <Group gap="sm">
          <Text component={Link} to="/kids" c="blue" size="sm">{t('kids.back')}</Text>
        </Group>

        <HeroStat label={balance.name} value={<MoneyText cents={balance.totalCents} span fw={800} size="2.5rem" />} />
        <Group justify="center" gap="lg">
          <Stack gap={0} align="center">
            <Text size="xs" c="dimmed">{t('kids.ours')}</Text>
            <MoneyText cents={balance.oursCents} fw={600} />
          </Stack>
          <Stack gap={0} align="center">
            <Text size="xs" c="dimmed">{t('kids.theirs')}</Text>
            <MoneyText cents={balance.theirsCents} fw={600} />
          </Stack>
        </Group>

        {chartBlock}
        {reportedBlock}
        {isAdmin && (
          <Button variant="light" size="xs" onClick={openReportedSheet}>{t('kids.updateReportedBalance')}</Button>
        )}

        <Group justify="space-between">
          <Text fw={600} size="sm">{t('kids.ledger')}</Text>
          <Button size="xs" leftSection={<IconPlus size={14} />} onClick={() => setAddSheetOpen(true)}>{t('kids.booking')}</Button>
        </Group>

        {entries.length === 0 ? (
          <EmptyState message={t('kids.noEntries')} />
        ) : (
          <MobileList>
            {[...entries].reverse().map((e) => (
              <MobileListRow
                key={e.id}
                leftSection={<Box w={10} h={10} bg={`${ENTRY_TYPE_COLORS[e.entryType] ?? 'gray'}.6`} style={{ borderRadius: '50%', flexShrink: 0 }} />}
                title={e.description || t(`kids.entry_${e.entryType}`)}
                subtitle={`${e.entryDate ? dayjs(e.entryDate).format('DD-MM-YYYY') : ''} · ${t(`kids.${e.owner}`)}`}
                trailing={
                  <Stack gap={0} align="flex-end">
                    <MoneyText cents={e.amountCents} colored fw={600} size="sm" />
                  </Stack>
                }
                chevron={MANUAL_ENTRY_TYPES.includes(e.entryType)}
                onClick={MANUAL_ENTRY_TYPES.includes(e.entryType) ? () => startEditEntry(e.id, e.owner, e.entryType, e.amountCents) : undefined}
                swipeAction={MANUAL_ENTRY_TYPES.includes(e.entryType) ? {
                  label: <IconTrash size={18} />,
                  destructive: true,
                  onTrigger: () => deleteEntry.mutate(e.id),
                } : undefined}
              />
            ))}
          </MobileList>
        )}

        <BottomSheet opened={addSheetOpen} onClose={() => setAddSheetOpen(false)} title={t('kids.booking')}>
          {ownerControl}
          <Select
            label={t('kids.type')}
            data={MANUAL_ENTRY_TYPES.map((v) => ({ value: v, label: t(`kids.entry_${v}`) }))}
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
            label={t('kids.description')}
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
          <Button loading={createEntry.isPending} onClick={handleAdd}>{t('kids.book')}</Button>
        </BottomSheet>

        <BottomSheet opened={editingEntryId !== null} onClose={() => setEditingEntryId(null)} title={t('common.edit')}>
          <SegmentedControl
            value={editOwner}
            onChange={(v) => setEditOwner(v as 'ours' | 'theirs')}
            data={[
              { label: t('kids.ours'), value: 'ours' },
              { label: t('kids.theirs'), value: 'theirs' },
            ]}
          />
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

        <BottomSheet opened={reportedSheetOpen} onClose={() => setReportedSheetOpen(false)} title={t('kids.updateReportedBalance')}>
          <DateInput
            label={t('common.date')}
            value={reportedDate}
            onChange={setReportedDate}
            valueFormat="DD-MM-YYYY"
          />
          <NumberInput
            label={t('kids.reportedBalance')}
            value={reportedAmount}
            onChange={setReportedAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
            placeholder="0,00"
          />
          <Button loading={updateReported.isPending} onClick={saveReported}>{t('common.save')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Group gap="sm">
          <Text component={Link} to="/kids" c="blue" size="sm">{t('kids.back')}</Text>
          <Title order={2}>{balance.name}</Title>
        </Group>
        <Group gap="lg">
          <Stack gap={0} align="flex-end">
            <Text size="xs" c="dimmed">{t('kids.ours')}</Text>
            <MoneyText cents={balance.oursCents} fw={600} />
          </Stack>
          <Stack gap={0} align="flex-end">
            <Text size="xs" c="dimmed">{t('kids.theirs')}</Text>
            <MoneyText cents={balance.theirsCents} fw={600} />
          </Stack>
          <MoneyText cents={balance.totalCents} size="xl" fw={800} />
        </Group>
      </Group>

      {chartBlock}
      {reportedBlock}
      {isAdmin && (
        <Group justify="flex-end">
          <Button variant="light" size="xs" onClick={openReportedSheet}>{t('kids.updateReportedBalance')}</Button>
        </Group>
      )}

      <Paper shadow="xs" p="md" withBorder>
        <Title order={4} mb="sm">{t('kids.ledger')}</Title>

        <Group gap="xs" align="flex-end" mb="md">
          {ownerControl}
          <Select
            label={t('kids.type')}
            data={MANUAL_ENTRY_TYPES.map((v) => ({ value: v, label: t(`kids.entry_${v}`) }))}
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
            label={t('kids.description')}
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
          <ActionIcon size="lg" variant="filled" aria-label={t('kids.book')} loading={createEntry.isPending} onClick={handleAdd}>+</ActionIcon>
        </Group>

        {entries.length === 0 ? (
          <EmptyState message={t('kids.noEntries')} />
        ) : (
          <Table.ScrollContainer minWidth={750}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('common.date')}</Table.Th>
                <Table.Th>{t('kids.owner')}</Table.Th>
                <Table.Th>{t('kids.type')}</Table.Th>
                <Table.Th>{t('kids.description')}</Table.Th>
                <Table.Th ta="right">{t('common.amount')}</Table.Th>
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
                    {editingEntryId === e.id ? (
                      <SegmentedControl
                        size="xs"
                        value={editOwner}
                        onChange={(v) => setEditOwner(v as 'ours' | 'theirs')}
                        data={[
                          { label: t('kids.ours'), value: 'ours' },
                          { label: t('kids.theirs'), value: 'theirs' },
                        ]}
                      />
                    ) : (
                      <Badge size="xs" variant="light" color={e.owner === 'ours' ? 'blue' : 'green'}>
                        {t(`kids.${e.owner}`)}
                      </Badge>
                    )}
                  </Table.Td>
                  <Table.Td>
                    <Badge size="xs" variant="light" color={ENTRY_TYPE_COLORS[e.entryType] ?? 'gray'}>
                      {t(`kids.entry_${e.entryType}`)}
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
                  <Table.Td>
                    {MANUAL_ENTRY_TYPES.includes(e.entryType) && editingEntryId !== e.id && (
                      <Group gap={4} wrap="nowrap">
                        <ActionIcon size="sm" variant="subtle" aria-label={t('common.edit')} onClick={() => startEditEntry(e.id, e.owner, e.entryType, e.amountCents)}><IconPencil size={14} /></ActionIcon>
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

      <BottomSheet opened={reportedSheetOpen} onClose={() => setReportedSheetOpen(false)} title={t('kids.updateReportedBalance')}>
        <DateInput
          label={t('common.date')}
          value={reportedDate}
          onChange={setReportedDate}
          valueFormat="DD-MM-YYYY"
        />
        <NumberInput
          label={t('kids.reportedBalance')}
          value={reportedAmount}
          onChange={setReportedAmount}
          decimalSeparator=","
          decimalScale={2}
          prefix="€ "
          hideControls
          placeholder="0,00"
        />
        <Button loading={updateReported.isPending} onClick={saveReported}>{t('common.save')}</Button>
      </BottomSheet>
    </Stack>
  )
}
