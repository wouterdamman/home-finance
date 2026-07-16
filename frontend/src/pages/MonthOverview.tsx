import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import {
  Title, Text, Group, Button, Badge, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, Paper, TextInput, Progress, Select, Menu,
} from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { modals } from '@mantine/modals'
import { IconTrash, IconX, IconDotsVertical, IconPlus } from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useMonthOverview, useClosePeriod, useReopenPeriod, useUpdateBudgetLine, useCreateBudgetLine, useDeletePeriod } from '../api/hooks/usePeriods'
import { useUpdateIncome, useDeleteIncome, useCreateIncome } from '../api/hooks/useIncomes'
import { useReplaceSplits } from '../api/hooks/useSplits'
import { useCategories, usePots } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import PasswordModal from '../components/PasswordModal'
import { parseToCents } from '../lib/money'
import { getErrorMessage } from '../api/client'
import HeroStat from '../components/mobile/HeroStat'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import BottomSheet from '../components/mobile/BottomSheet'

const MONTH_NL = ['','Januari','Februari','Maart','April','Mei','Juni','Juli','Augustus','September','Oktober','November','December']
const MONTH_EN = ['','January','February','March','April','May','June','July','August','September','October','November','December']

function parseCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
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

  const navigate = useNavigate()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const [splitEdits, setSplitEdits] = useState<Record<number, string> | null>(null)
  const [newLabel, setNewLabel] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newCategoryId, setNewCategoryId] = useState<string | null>(null)
  const [newBudgetAmount, setNewBudgetAmount] = useState<number | string>('')
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [incomeSheet, setIncomeSheet] = useState<'create' | number | null>(null)
  const [editIncomeAmount, setEditIncomeAmount] = useState<number | string>('')
  const [budgetSheetOpen, setBudgetSheetOpen] = useState(false)

  const closePeriod = useClosePeriod(periodId ?? 0, y, m)
  const reopenPeriod = useReopenPeriod(periodId ?? 0, y, m)
  const createBudgetLine = useCreateBudgetLine(periodId ?? 0, y)
  const updateBudgetLine = useUpdateBudgetLine(periodId ?? 0, y)
  const deletePeriod = useDeletePeriod(periodId ?? 0, y)
  const createIncome = useCreateIncome(periodId ?? 0)
  const updateIncome = useUpdateIncome(periodId ?? 0)
  const deleteIncome = useDeleteIncome(periodId ?? 0)
  const replaceSplits = useReplaceSplits(periodId ?? 0)
  const { data: categories } = useCategories()
  const { data: potsList } = usePots()
  const [newSplitPotId, setNewSplitPotId] = useState<string | null>(null)

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

  const categoryById = new Map((categories ?? []).map(c => [c.id, c] as const))
  const budgetLineLabel = (bl: typeof budgetLines[number]) =>
    bl.label ?? (bl.categoryId != null ? categoryById.get(bl.categoryId)?.name : undefined) ?? '—'
  const usedCategoryIds = new Set(budgetLines.map(bl => bl.categoryId).filter((id): id is number => id != null))
  const availableCategories = (categories ?? []).filter(c => !c.archivedAt && !usedCategoryIds.has(c.id))

  const handleAddBudgetLine = () => {
    if (!newCategoryId) return
    const cat = categoryById.get(Number(newCategoryId))
    createBudgetLine.mutate({
      categoryId: Number(newCategoryId),
      amountCents: newBudgetAmount !== '' ? parseCents(newBudgetAmount) : (cat?.defaultAmountCents ?? 0),
      tracksTransactions: cat?.isItemized ?? false,
      sortOrder: budgetLines.length,
    }, { onSuccess: () => { setNewCategoryId(null); setNewBudgetAmount('') } })
  }

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

  const handleToggleTracking = (bl: { id: number; label?: string; amountCents: number; tracksTransactions: boolean; sortOrder: number }) => modals.openConfirmModal({
    title: t('month.toggleTracking'),
    children: <Text size="sm">{bl.tracksTransactions ? t('month.toggleTrackingOffConfirm') : t('month.toggleTrackingOnConfirm')}</Text>,
    labels: { confirm: t('common.confirm'), cancel: t('common.cancel') },
    onConfirm: () => updateBudgetLine.mutate({
      id: bl.id, label: bl.label ?? null, amountCents: bl.amountCents,
      tracksTransactions: !bl.tracksTransactions, sortOrder: bl.sortOrder,
    }),
  })

  const carryoverPot = (potsList ?? []).find(p => p.kind === 'carryover' && !p.archivedAt)
  const currentSplits = splitEdits ?? Object.fromEntries(splits.map(s => [s.potId, s.percentage]))
  const potNameById = new Map((potsList ?? []).map(p => [p.id, p.name] as const))
  const baseSplitPotIds = splitEdits ? Object.keys(splitEdits).map(Number) : splits.map(s => s.potId)
  const splitPotIds = splitEdits && carryoverPot && !baseSplitPotIds.includes(carryoverPot.id)
    ? [...baseSplitPotIds, carryoverPot.id]
    : baseSplitPotIds
  const availablePotsToAdd = (potsList ?? []).filter(p => !p.archivedAt && p.kind !== 'carryover' && !splitPotIds.includes(p.id))
  const nonCarryoverTotal = carryoverPot
    ? Object.entries(currentSplits).reduce((sum, [potId, pct]) => Number(potId) === carryoverPot.id ? sum : sum + (Number(pct) || 0), 0)
    : 0
  const carryoverRemainder = 100 - nonCarryoverTotal
  const carryoverOverAllocated = !!carryoverPot && carryoverRemainder < -0.005

  const handleSaveSplits = () => {
    replaceSplits.mutate(
      Object.entries(currentSplits).map(([potId, pct]) => ({ potId: Number(potId), percentage: pct })),
      { onSuccess: () => setSplitEdits(null) }
    )
  }

  const handleAddSplitPot = () => {
    if (!newSplitPotId) return
    setSplitEdits(prev => ({ ...(prev ?? {}), [newSplitPotId]: '0' }))
    setNewSplitPotId(null)
  }

  const handleRemoveSplitPot = (potId: number) => {
    setSplitEdits(prev => {
      const next = { ...(prev ?? {}) }
      delete next[potId]
      return next
    })
  }

  if (isMobile) {
    const editingIncome = typeof incomeSheet === 'number' ? incomes.find(inc => inc.id === incomeSheet) : undefined

    const handleSaveIncomeEdit = () => {
      if (!editingIncome) return
      const cents = parseCents(editIncomeAmount)
      updateIncome.mutate(
        { id: editingIncome.id, amountCents: cents, notes: editingIncome.notes, sortOrder: editingIncome.sortOrder, label: editingIncome.label },
        { onSuccess: () => setIncomeSheet(null) }
      )
    }

    const handleCreateIncome = () => {
      createIncome.mutate({ label: newLabel, amountCents: parseCents(newAmount), notes: '', sortOrder: incomes.length }, {
        onSuccess: () => { setNewLabel(''); setNewAmount(''); setIncomeSheet(null) }
      })
    }

    return (
      <Stack gap="md">
        <Group justify="space-between" wrap="nowrap">
          <Group gap="sm">
            <Text component={Link} to={`/years/${y}`} c="blue" size="sm">← {y}</Text>
            <Badge color={isClosed ? 'green' : 'orange'}>{isClosed ? t('month.statusClosed') : t('month.statusOpen')}</Badge>
          </Group>
          <Menu position="bottom-end">
            <Menu.Target>
              <ActionIcon variant="subtle" aria-label={t('common.edit')}><IconDotsVertical size={18} /></ActionIcon>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Item component={Link} to={`/months/${y}/${m}/transactions`}>{t('month.transactions')}</Menu.Item>
              {isClosed
                ? <Menu.Item color="orange" onClick={handleReopen}>{t('month.reopenAction')}</Menu.Item>
                : <Menu.Item color="green" onClick={handleClose}>{t('month.closeAction')}</Menu.Item>}
              {!isClosed && <Menu.Item color="red" onClick={() => setDeleteOpen(true)}>{t('month.delete')}</Menu.Item>}
            </Menu.Dropdown>
          </Menu>
        </Group>
        <Title order={3}>{monthName} {y}</Title>

        <HeroStat label={t('month.surplusLabel')} value={<MoneyText cents={surplusCents} span fw={800} size="2.5rem" colored />} />

        <Stack gap="xs">
          <Text fw={600} size="sm">{t('month.income')}</Text>
          <MobileList>
            {incomes.map(inc => (
              <MobileListRow
                key={inc.id}
                title={inc.label ?? t('month.unknownSource', { id: inc.sourceId })}
                subtitle={inc.entryType === 'carryover' ? t('month.carryoverBadge') : undefined}
                trailing={<MoneyText cents={inc.amountCents} fw={600} />}
                chevron={!isClosed && inc.entryType !== 'carryover'}
                onClick={!isClosed && inc.entryType !== 'carryover' ? () => { setIncomeSheet(inc.id); setEditIncomeAmount(inc.amountCents / 100) } : undefined}
              />
            ))}
            {!isClosed && (
              <MobileListRow
                title={t('month.addIncome')}
                leftSection={<IconPlus size={16} />}
                onClick={() => { setNewLabel(''); setNewAmount(''); setIncomeSheet('create') }}
              />
            )}
          </MobileList>
          <Group justify="space-between" px="xs">
            <Text fw={700} size="sm">{t('month.totalIncome')}</Text>
            <MoneyText cents={incomeTotalCents} fw={700} />
          </Group>
        </Stack>

        <Stack gap="xs">
          <Text fw={600} size="sm">{t('month.expenses')}</Text>
          <MobileList>
            {budgetLines.map(bl => {
              const budgeted = bl.tracksTransactions && bl.amountCents > 0
              const pct = budgeted ? (bl.effectiveCents / bl.amountCents) * 100 : 0
              const progressColor = pct > 100 ? 'red' : pct >= 80 ? 'orange' : 'green'
              return (
                <MobileListRow
                  key={bl.id}
                  title={budgetLineLabel(bl)}
                  subtitle={budgeted ? <Progress value={Math.min(pct, 100)} color={progressColor} size="sm" mt={2} /> : undefined}
                  to={bl.tracksTransactions ? `/months/${y}/${m}/transactions?category=${bl.categoryId}` : undefined}
                  chevron={bl.tracksTransactions}
                  trailing={
                    <Group gap={6} wrap="nowrap">
                      <MoneyText cents={bl.effectiveCents} fw={600} />
                      {!isClosed && (
                        <ActionIcon
                          size="sm"
                          variant={bl.tracksTransactions ? 'filled' : 'subtle'}
                          color={bl.tracksTransactions ? 'blue' : 'gray'}
                          aria-label={t('month.toggleTracking')}
                          onClick={(e) => { e.preventDefault(); e.stopPropagation(); handleToggleTracking(bl) }}
                        >≡</ActionIcon>
                      )}
                    </Group>
                  }
                />
              )
            })}
            {!isClosed && availableCategories.length > 0 && (
              <MobileListRow title={t('month.addCategory')} leftSection={<IconPlus size={16} />} onClick={() => setBudgetSheetOpen(true)} />
            )}
          </MobileList>
          <Group justify="space-between" px="xs">
            <Text fw={700} size="sm">{t('month.totalExpenses')}</Text>
            <MoneyText cents={expenseTotalCents} fw={700} />
          </Group>
        </Stack>

        <Stack gap="xs">
          <Group justify="space-between">
            <Text fw={600} size="sm">{t('month.splitSection')}</Text>
            {!isClosed && !splitEdits && (
              <Button size="xs" variant="subtle" onClick={() => setSplitEdits(Object.fromEntries(splits.map(s => [s.potId, s.percentage])))}>
                {t('month.adjust')}
              </Button>
            )}
          </Group>
          <MobileList>
            {splitPotIds.map(potId => {
              const sp = splits.find(s => s.potId === potId)
              const name = sp?.potName ?? potNameById.get(potId) ?? '—'
              const isCarryoverRow = !!carryoverPot && potId === carryoverPot.id
              return (
                <MobileListRow
                  key={potId}
                  title={name}
                  subtitle={`${sp?.percentage ?? 0}%${isCarryoverRow ? ` · ${t('pots.kind_carryover')}` : ''}`}
                  trailing={sp ? <MoneyText cents={sp.projectedCents ?? 0} colored={!isClosed} fw={600} /> : undefined}
                />
              )
            })}
          </MobileList>
        </Stack>

        <BottomSheet opened={incomeSheet !== null} onClose={() => setIncomeSheet(null)} title={incomeSheet === 'create' ? t('month.addIncome') : t('common.edit')}>
          {incomeSheet === 'create' ? (
            <>
              <TextInput label={t('common.description')} value={newLabel} onChange={e => setNewLabel(e.target.value)} />
              <NumberInput label={t('common.amount')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls placeholder="0,00" />
              <Button disabled={!newLabel} loading={createIncome.isPending} onClick={handleCreateIncome}>{t('common.add')}</Button>
            </>
          ) : editingIncome && (
            <>
              <NumberInput
                label={t('common.amount')}
                value={editIncomeAmount}
                onChange={setEditIncomeAmount}
                decimalSeparator=","
                decimalScale={2}
                prefix="€ "
                hideControls
              />
              <Group grow>
                <Button onClick={handleSaveIncomeEdit} loading={updateIncome.isPending}>{t('common.save')}</Button>
                <Button
                  color="red"
                  variant="light"
                  loading={deleteIncome.isPending}
                  onClick={() => deleteIncome.mutate(editingIncome.id, { onSuccess: () => setIncomeSheet(null) })}
                >
                  {t('common.delete')}
                </Button>
              </Group>
            </>
          )}
        </BottomSheet>

        <BottomSheet opened={budgetSheetOpen} onClose={() => setBudgetSheetOpen(false)} title={t('month.addCategory')}>
          <Select
            label={t('settings.categories')}
            placeholder={t('month.addCategory')}
            data={availableCategories.map(c => ({ value: String(c.id), label: c.name }))}
            value={newCategoryId}
            onChange={setNewCategoryId}
            searchable
            clearable
          />
          <NumberInput label={t('common.amount')} value={newBudgetAmount} onChange={setNewBudgetAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls placeholder="0,00" />
          <Button disabled={!newCategoryId} loading={createBudgetLine.isPending} onClick={() => { handleAddBudgetLine(); setBudgetSheetOpen(false) }}>
            {t('common.add')}
          </Button>
        </BottomSheet>

        <BottomSheet opened={!!splitEdits} onClose={() => setSplitEdits(null)} title={t('month.splitSection')}>
          {carryoverOverAllocated && <Text size="xs" c="red">{t('month.splitOverAllocated')}</Text>}
          <Stack gap="sm">
            {splitPotIds.map(potId => {
              const sp = splits.find(s => s.potId === potId)
              const name = sp?.potName ?? potNameById.get(potId) ?? '—'
              const isCarryoverRow = !!carryoverPot && potId === carryoverPot.id
              return (
                <Group key={potId} justify="space-between" wrap="nowrap">
                  <Text size="sm" style={{ flex: 1 }}>{name}{isCarryoverRow && <Text span size="xs" c="dimmed"> ({t('pots.kind_carryover')})</Text>}</Text>
                  {isCarryoverRow ? (
                    <Text size="sm" c={carryoverOverAllocated ? 'red' : 'dimmed'}>{carryoverRemainder.toFixed(2)}%</Text>
                  ) : (
                    <NumberInput
                      size="sm"
                      value={Number(splitEdits?.[potId] ?? sp?.percentage ?? 0)}
                      onChange={(v) => setSplitEdits(prev => ({ ...prev!, [potId]: String(v) }))}
                      suffix="%"
                      decimalScale={2}
                      hideControls
                      min={0}
                      max={100}
                      w={100}
                    />
                  )}
                  {!isCarryoverRow && (
                    <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => handleRemoveSplitPot(potId)}><IconX size={14} /></ActionIcon>
                  )}
                </Group>
              )
            })}
            {availablePotsToAdd.length > 0 && (
              <Group wrap="nowrap">
                <Select
                  size="sm"
                  placeholder={t('month.addPot')}
                  data={availablePotsToAdd.map(p => ({ value: String(p.id), label: p.name }))}
                  value={newSplitPotId}
                  onChange={setNewSplitPotId}
                  searchable
                  clearable
                  style={{ flex: 1 }}
                />
                <Button size="sm" disabled={!newSplitPotId} onClick={handleAddSplitPot}>{t('common.add')}</Button>
              </Group>
            )}
            <Group grow>
              <Button onClick={handleSaveSplits} loading={replaceSplits.isPending} disabled={carryoverOverAllocated}>{t('common.save')}</Button>
              <Button variant="subtle" onClick={() => setSplitEdits(null)}>{t('common.cancel')}</Button>
            </Group>
          </Stack>
        </BottomSheet>

        <PasswordModal
          opened={deleteOpen}
          onClose={() => setDeleteOpen(false)}
          title={t('month.deleteTitle')}
          warningText={t('month.deleteWarning', { month: monthName, year: y })}
          confirmLabel={t('month.deleteConfirm')}
          confirmColor="red"
          loading={deletePeriod.isPending}
          onConfirm={handleDelete}
        />
      </Stack>
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
          {!isClosed && (
            <Button color="red" variant="subtle" size="sm" onClick={() => setDeleteOpen(true)}>
              {t('month.delete')}
            </Button>
          )}
          {isClosed
            ? <Button color="orange" onClick={handleReopen} loading={reopenPeriod.isPending}>{t('month.reopenAction')}</Button>
            : <Button color="green" onClick={handleClose} loading={closePeriod.isPending}>{t('month.closeAction')}</Button>
          }
        </Group>
      </Group>

      {/* Incomes */}
      <Paper shadow="xs" p="md" withBorder>
        <Title order={4} mb="sm">{t('month.income')}</Title>
        <Table.ScrollContainer minWidth={420}>
        <Table>
          <Table.Tbody>
            {incomes.map(inc => (
              <Table.Tr key={inc.id}>
                <Table.Td>
                  <Text size="sm" c={inc.entryType === 'carryover' ? 'dimmed' : undefined}>
                    {inc.label ?? t('month.unknownSource', { id: inc.sourceId })}
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
                          const newCents = parseCents(e.target.value)
                          if (newCents !== inc.amountCents)
                            updateIncome.mutate({ id: inc.id, amountCents: newCents, notes: inc.notes, sortOrder: inc.sortOrder, label: inc.label })
                        }}
                      />
                  }
                </Table.Td>
                {!isClosed && inc.entryType !== 'carryover' && (
                  <Table.Td w={40}>
                    <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => deleteIncome.mutate(inc.id)}><IconTrash size={14} /></ActionIcon>
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
                  <Button size="xs" aria-label={t('common.add')} disabled={!newLabel} loading={createIncome.isPending} onClick={() => {
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
        </Table.ScrollContainer>
      </Paper>

      {/* Budget lines */}
      <Paper shadow="xs" p="md" withBorder>
        <Title order={4} mb="sm">{t('month.expenses')}</Title>
        <Table.ScrollContainer minWidth={420}>
        <Table>
          <Table.Tbody>
            {budgetLines.map(bl => {
              const budgeted = bl.tracksTransactions && bl.amountCents > 0
              const pct = budgeted ? (bl.effectiveCents / bl.amountCents) * 100 : 0
              const progressColor = pct > 100 ? 'red' : pct >= 80 ? 'orange' : 'green'
              return (
              <Table.Tr key={bl.id}>
                <Table.Td>
                  {bl.tracksTransactions
                    ? <Text component={Link} to={`/months/${y}/${m}/transactions?category=${bl.categoryId}`} c="blue" size="sm">{budgetLineLabel(bl)}</Text>
                    : <Text size="sm">{budgetLineLabel(bl)}</Text>
                  }
                </Table.Td>
                <Table.Td ta="right">
                  <MoneyText cents={bl.effectiveCents} />
                  {bl.tracksTransactions && bl.amountCents > 0 && (
                    <Text size="xs" c="dimmed" span> / {(bl.amountCents / 100).toFixed(2)}</Text>
                  )}
                  {budgeted && (
                    <Stack gap={2} mt={4} align="flex-end">
                      <Progress value={Math.min(pct, 100)} color={progressColor} size="sm" w="100%" />
                      <Text size="xs" c="dimmed">
                        {pct.toFixed(0)}% — {t('month.remaining')}: <MoneyText cents={bl.amountCents - bl.effectiveCents} colored span size="xs" />
                      </Text>
                    </Stack>
                  )}
                </Table.Td>
                {!isClosed && (
                  <Table.Td w={32}>
                    <ActionIcon
                      size="xs"
                      variant={bl.tracksTransactions ? 'filled' : 'subtle'}
                      color={bl.tracksTransactions ? 'blue' : 'gray'}
                      title={t('month.toggleTracking')}
                      aria-label={t('month.toggleTracking')}
                      onClick={() => handleToggleTracking(bl)}
                    >≡</ActionIcon>
                  </Table.Td>
                )}
              </Table.Tr>
              )
            })}
          </Table.Tbody>
          <Table.Tfoot>
            {!isClosed && availableCategories.length > 0 && (
              <Table.Tr>
                <Table.Td>
                  <Select
                    size="xs"
                    placeholder={t('month.addCategory')}
                    data={availableCategories.map(c => ({ value: String(c.id), label: c.name }))}
                    value={newCategoryId}
                    onChange={setNewCategoryId}
                    searchable
                    clearable
                  />
                </Table.Td>
                <Table.Td>
                  <NumberInput
                    size="xs"
                    value={newBudgetAmount}
                    onChange={setNewBudgetAmount}
                    decimalSeparator=","
                    decimalScale={2}
                    prefix="€ "
                    hideControls
                    placeholder="0,00"
                  />
                </Table.Td>
                <Table.Td>
                  <Button size="xs" aria-label={t('common.add')} disabled={!newCategoryId} loading={createBudgetLine.isPending} onClick={handleAddBudgetLine}>+</Button>
                </Table.Td>
              </Table.Tr>
            )}
            <Table.Tr fw={700}>
              <Table.Td>{t('month.totalExpenses')}</Table.Td>
              <Table.Td ta="right"><MoneyText cents={expenseTotalCents} /></Table.Td>
            </Table.Tr>
          </Table.Tfoot>
        </Table>
        </Table.ScrollContainer>
      </Paper>

      {/* Surplus banner */}
      <Paper
        shadow="xs"
        p="md"
        withBorder
        style={{
          background: surplusCents >= 0
            ? 'light-dark(var(--mantine-color-green-0), var(--mantine-color-green-9))'
            : 'light-dark(var(--mantine-color-red-0), var(--mantine-color-red-9))',
        }}
      >
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
                  <Button size="xs" onClick={handleSaveSplits} loading={replaceSplits.isPending} disabled={carryoverOverAllocated}>{t('common.save')}</Button>
                  <Button size="xs" variant="subtle" onClick={() => setSplitEdits(null)}>{t('common.cancel')}</Button>
                </Group>
              : <Button size="xs" variant="subtle" onClick={() => setSplitEdits(Object.fromEntries(splits.map(s => [s.potId, s.percentage])))}>
                  {t('month.adjust')}
                </Button>
          )}
        </Group>
        {carryoverOverAllocated && (
          <Text size="xs" c="red" mb="sm">{t('month.splitOverAllocated')}</Text>
        )}
        <Table.ScrollContainer minWidth={420}>
        <Table>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('month.pot')}</Table.Th>
              <Table.Th ta="right">{t('month.percentage')}</Table.Th>
              <Table.Th ta="right">{t('month.amount')}</Table.Th>
              {splitEdits && <Table.Th w={32} />}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {splitPotIds.map(potId => {
              const sp = splits.find(s => s.potId === potId)
              const name = sp?.potName ?? potNameById.get(potId) ?? '—'
              const isCarryoverRow = !!carryoverPot && potId === carryoverPot.id
              return (
              <Table.Tr key={potId}>
                <Table.Td>
                  <Text size="sm">{name}</Text>
                  {isCarryoverRow && <Badge size="xs" ml="xs" color="gray">{t('pots.kind_carryover')}</Badge>}
                </Table.Td>
                <Table.Td ta="right" w={120}>
                  {splitEdits
                    ? (isCarryoverRow
                        ? <Text size="sm" c={carryoverOverAllocated ? 'red' : 'dimmed'}>{carryoverRemainder.toFixed(2)}%</Text>
                        : <NumberInput
                            size="xs"
                            value={Number(splitEdits[potId] ?? sp?.percentage ?? 0)}
                            onChange={(v) => setSplitEdits(prev => ({ ...prev!, [potId]: String(v) }))}
                            suffix="%"
                            decimalScale={2}
                            hideControls
                            min={0}
                            max={100}
                          />
                      )
                    : <Text size="sm">{sp?.percentage}%</Text>
                  }
                </Table.Td>
                <Table.Td ta="right">
                  {sp ? <MoneyText cents={sp.projectedCents ?? 0} colored={!isClosed} /> : <Text size="sm" c="dimmed">—</Text>}
                </Table.Td>
                {splitEdits && (
                  <Table.Td>
                    {!isCarryoverRow && (
                      <ActionIcon color="red" size="sm" variant="subtle" aria-label={t('common.delete')} onClick={() => handleRemoveSplitPot(potId)}><IconX size={14} /></ActionIcon>
                    )}
                  </Table.Td>
                )}
              </Table.Tr>
              )
            })}
            {splitEdits && availablePotsToAdd.length > 0 && (
              <Table.Tr>
                <Table.Td colSpan={2}>
                  <Select
                    size="xs"
                    placeholder={t('month.addPot')}
                    data={availablePotsToAdd.map(p => ({ value: String(p.id), label: p.name }))}
                    value={newSplitPotId}
                    onChange={setNewSplitPotId}
                    searchable
                    clearable
                  />
                </Table.Td>
                <Table.Td colSpan={2}>
                  <Button size="xs" disabled={!newSplitPotId} onClick={handleAddSplitPot}>{t('common.add')}</Button>
                </Table.Td>
              </Table.Tr>
            )}
          </Table.Tbody>
        </Table>
        </Table.ScrollContainer>
      </Paper>
      <PasswordModal
        opened={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title={t('month.deleteTitle')}
        warningText={t('month.deleteWarning', { month: monthName, year: y })}
        confirmLabel={t('month.deleteConfirm')}
        confirmColor="red"
        loading={deletePeriod.isPending}
        onConfirm={handleDelete}
      />
    </Stack>
  )

  function handleDelete(password: string) {
    deletePeriod.mutate(password, {
      onSuccess: () => {
        setDeleteOpen(false)
        navigate(`/years/${y}`)
      },
      onError: (err: unknown) => {
        notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) })
      },
    })
  }
}
