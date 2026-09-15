import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import {
  Title, Text, Group, Button, Badge, Skeleton, Alert, Table,
  NumberInput, ActionIcon, Stack, Paper, Progress, Select, Menu, Modal,
} from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { modals } from '@mantine/modals'
import { IconTrash, IconX, IconDotsVertical, IconPlus, IconPencil, IconArrowsSort } from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useMonthOverview, useClosePeriod, useReopenPeriod, useUpdateBudgetLine, useCreateBudgetLine, useDeleteBudgetLine, useDeletePeriod } from '../api/hooks/usePeriods'
import { useMe } from '../api/hooks/useMe'
import { useUpdateIncome, useDeleteIncome, useCreateIncome } from '../api/hooks/useIncomes'
import { useReplaceSplits } from '../api/hooks/useSplits'
import { useCategories, usePots, useIncomeSources } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import ReauthConfirmModal from '../components/ReauthConfirmModal'
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

type SortMode = 'default' | 'name-asc' | 'name-desc' | 'amount-asc' | 'amount-desc'

function sortByMode<T>(items: T[], mode: SortMode, nameOf: (item: T) => string, amountOf: (item: T) => number): T[] {
  if (mode === 'default') return items
  const copy = [...items]
  copy.sort((a, b) => {
    if (mode === 'name-asc') return nameOf(a).localeCompare(nameOf(b))
    if (mode === 'name-desc') return nameOf(b).localeCompare(nameOf(a))
    if (mode === 'amount-asc') return amountOf(a) - amountOf(b)
    return amountOf(b) - amountOf(a)
  })
  return copy
}

function SortControl({ mode, onChange }: { mode: SortMode; onChange: (mode: SortMode) => void }) {
  const { t } = useTranslation()
  const options: { value: SortMode; label: string }[] = [
    { value: 'default', label: t('month.sortDefault') },
    { value: 'name-asc', label: t('month.sortNameAsc') },
    { value: 'name-desc', label: t('month.sortNameDesc') },
    { value: 'amount-asc', label: t('month.sortAmountAsc') },
    { value: 'amount-desc', label: t('month.sortAmountDesc') },
  ]
  return (
    <Menu position="bottom-end">
      <Menu.Target>
        <ActionIcon variant="subtle" color={mode === 'default' ? 'gray' : 'blue'} aria-label={t('common.sortBy')} title={t('common.sortBy')}>
          <IconArrowsSort size={16} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        {options.map(o => (
          <Menu.Item key={o.value} onClick={() => onChange(o.value)} fw={mode === o.value ? 700 : 400}>
            {o.label}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  )
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
  const isMobile = useMediaQuery('(max-width: 47.99em)', undefined, { getInitialValueInEffect: false })
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'
  const [splitEdits, setSplitEdits] = useState<Record<number, string> | null>(null)
  const [splitAmountEdits, setSplitAmountEdits] = useState<Record<number, string> | null>(null)
  const [newIncomeSourceId, setNewIncomeSourceId] = useState<string | null>(null)
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newCategoryId, setNewCategoryId] = useState<string | null>(null)
  const [newBudgetAmount, setNewBudgetAmount] = useState<number | string>('')
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editBudgetLineId, setEditBudgetLineId] = useState<number | null>(null)
  const [editBudgetAmount, setEditBudgetAmount] = useState<number | string>('')
  const [incomeSheet, setIncomeSheet] = useState<'create' | number | null>(null)
  const [editIncomeAmount, setEditIncomeAmount] = useState<number | string>('')
  const [budgetSheetOpen, setBudgetSheetOpen] = useState(false)
  const [incomeSort, setIncomeSort] = useState<SortMode>('amount-desc')
  const [expenseSort, setExpenseSort] = useState<SortMode>('amount-desc')
  const [splitSort, setSplitSort] = useState<SortMode>('amount-desc')

  const closePeriod = useClosePeriod(periodId ?? 0, y, m)
  const reopenPeriod = useReopenPeriod(periodId ?? 0, y, m)
  const createBudgetLine = useCreateBudgetLine(periodId ?? 0, y)
  const updateBudgetLine = useUpdateBudgetLine(periodId ?? 0, y)
  const deleteBudgetLine = useDeleteBudgetLine(periodId ?? 0, y)
  const deletePeriod = useDeletePeriod(periodId ?? 0, y)
  const createIncome = useCreateIncome(periodId ?? 0)
  const updateIncome = useUpdateIncome(periodId ?? 0)
  const deleteIncome = useDeleteIncome(periodId ?? 0)
  const replaceSplits = useReplaceSplits(periodId ?? 0)
  const { data: categories } = useCategories()
  const { data: incomeSources } = useIncomeSources()
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
  // Child categories never get their own budget_lines row — their
  // transactions roll up into their parent's total (see category_rollup) —
  // so they're not selectable here.
  const availableCategories = (categories ?? []).filter(c => !c.archivedAt && !c.parentId && !usedCategoryIds.has(c.id))
  const usedIncomeSourceIds = new Set(incomes.filter(inc => inc.entryType === 'normal').map(inc => inc.sourceId).filter((id): id is number => id != null))
  const availableIncomeSources = (incomeSources ?? []).filter(s => !usedIncomeSourceIds.has(s.id))
  const incomeSourceById = new Map((incomeSources ?? []).map(s => [s.id, s] as const))

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

  const handleDeleteBudgetLine = (bl: { id: number; label?: string }, categoryLabel: string) => modals.openConfirmModal({
    title: t('month.deleteBudgetLineTitle'),
    children: <Text size="sm">{t('month.deleteBudgetLineConfirm', { category: bl.label || categoryLabel })}</Text>,
    labels: { confirm: t('common.delete'), cancel: t('common.cancel') },
    confirmProps: { color: 'red' },
    onConfirm: () => deleteBudgetLine.mutate(bl.id, {
      onError: (err) => notifications.show({ color: 'red', message: getErrorMessage(err, t('common.error')) }),
    }),
  })

  const handleDeleteIncome = (inc: { id: number; label?: string; sourceId?: number }, sourceLabel: string, onSuccess?: () => void) => modals.openConfirmModal({
    title: t('month.deleteIncomeTitle'),
    children: <Text size="sm">{t('month.deleteIncomeConfirm', { source: inc.label || sourceLabel })}</Text>,
    labels: { confirm: t('common.delete'), cancel: t('common.cancel') },
    confirmProps: { color: 'red' },
    onConfirm: () => deleteIncome.mutate(inc.id, {
      onSuccess,
      onError: (err) => notifications.show({ color: 'red', message: getErrorMessage(err, t('common.error')) }),
    }),
  })

  const openEditBudgetAmount = (bl: { id: number; amountCents: number }) => {
    setEditBudgetLineId(bl.id)
    setEditBudgetAmount(bl.amountCents / 100)
  }
  const handleSaveBudgetAmount = () => {
    const bl = budgetLines.find(b => b.id === editBudgetLineId)
    if (!bl) return
    updateBudgetLine.mutate({
      id: bl.id, label: bl.label ?? null, amountCents: parseCents(editBudgetAmount),
      tracksTransactions: bl.tracksTransactions, sortOrder: bl.sortOrder,
    }, { onSuccess: () => setEditBudgetLineId(null) })
  }

  const carryoverPot = (potsList ?? []).find(p => p.kind === 'carryover' && !p.archivedAt)
  const currentSplits = splitEdits ?? Object.fromEntries(splits.map(s => [s.potId, s.percentage]))
  const potNameById = new Map((potsList ?? []).map(p => [p.id, p.name] as const))
  const baseSplitPotIds = splitEdits ? Object.keys(splitEdits).map(Number) : splits.map(s => s.potId)
  const splitPotIds = splitEdits && carryoverPot && !baseSplitPotIds.includes(carryoverPot.id)
    ? [...baseSplitPotIds, carryoverPot.id]
    : baseSplitPotIds
  const availablePotsToAdd = (potsList ?? []).filter(p => !p.archivedAt && p.kind !== 'carryover' && !splitPotIds.includes(p.id))
  const potSplitName = (potId: number) => splits.find(s => s.potId === potId)?.potName ?? potNameById.get(potId) ?? '—'
  const potSplitAmount = (potId: number) => splits.find(s => s.potId === potId)?.projectedCents ?? 0
  const sortedIncomes = sortByMode(incomes, incomeSort, inc => inc.label ?? t('month.unknownSource', { id: inc.sourceId }), inc => inc.effectiveCents)
  const sortedBudgetLines = sortByMode(budgetLines, expenseSort, budgetLineLabel, bl => bl.effectiveCents)
  const sortedSplitPotIds = sortByMode(splitPotIds, splitSort, potSplitName, potSplitAmount)
  const nonCarryoverTotal = carryoverPot
    ? Object.entries(currentSplits).reduce((sum, [potId, pct]) => Number(potId) === carryoverPot.id ? sum : sum + (Number(pct) || 0), 0)
    : 0
  const carryoverRemainder = 100 - nonCarryoverTotal
  const carryoverOverAllocated = !!carryoverPot && carryoverRemainder < -0.005

  const currentAmountEdits = splitAmountEdits ?? Object.fromEntries(splits.map(s => [s.potId, String((s.projectedCents ?? 0) / 100)]))
  const pctToProjectedCents = (pct: number) => Math.round((surplusCents * pct) / 100)
  const centsToPct = (cents: number) => surplusCents !== 0 ? (cents / surplusCents) * 100 : 0

  // The percentage and amount inputs are two independently-controlled fields
  // that mirror each other: each one's value comes only from its own edit
  // state, and editing one derives an update to the *other* field's state as
  // a side effect. Deriving a field's own displayed value from a round-trip
  // through the other field on every keystroke fights the caret mid-typing
  // (Mantine reformats the controlled value each render) and garbles input.
  // Stores the raw string Mantine emits, exactly like the amount field below:
  // coercing through Number() here would snap a cleared field to 0 and swallow
  // a half-typed decimal separator mid-edit.
  const setSplitPercentage = (potId: number, text: string) => {
    setSplitEdits(prev => ({ ...prev!, [potId]: text }))
    const pct = Number(text)
    setSplitAmountEdits(prev => ({
      ...(prev ?? currentAmountEdits),
      [potId]: Number.isFinite(pct) ? String(pctToProjectedCents(pct) / 100) : '0',
    }))
  }
  // A zero surplus makes every amount 0%, a negative one flips the sign (€50
  // against a −€100 surplus stores −50%, which the server then rejects), so the
  // amount field is disabled rather than silently writing a nonsense split.
  const amountSplitDisabled = surplusCents <= 0
  const setSplitAmountText = (potId: number, text: string, amountValue: number) => {
    setSplitAmountEdits(prev => ({ ...(prev ?? currentAmountEdits), [potId]: text }))
    const pct = Math.round(centsToPct(Math.round(amountValue * 100)) * 100) / 100
    setSplitEdits(prev => ({ ...prev!, [potId]: String(pct) }))
  }

  const startSplitEdit = () => {
    setSplitEdits(Object.fromEntries(splits.map(s => [s.potId, s.percentage])))
    setSplitAmountEdits(Object.fromEntries(splits.map(s => [s.potId, String((s.projectedCents ?? 0) / 100)])))
  }
  const cancelSplitEdit = () => { setSplitEdits(null); setSplitAmountEdits(null) }

  const handleSaveSplits = () => {
    replaceSplits.mutate(
      Object.entries(currentSplits).map(([potId, pct]) => ({ potId: Number(potId), percentage: pct })),
      { onSuccess: () => { setSplitEdits(null); setSplitAmountEdits(null) } }
    )
  }

  const handleAddSplitPot = () => {
    if (!newSplitPotId) return
    setSplitEdits(prev => ({ ...(prev ?? {}), [newSplitPotId]: '0' }))
    setSplitAmountEdits(prev => ({ ...(prev ?? currentAmountEdits), [newSplitPotId]: '0' }))
    setNewSplitPotId(null)
  }

  const handleRemoveSplitPot = (potId: number) => {
    setSplitEdits(prev => {
      const next = { ...(prev ?? {}) }
      delete next[potId]
      return next
    })
    setSplitAmountEdits(prev => {
      const next = { ...(prev ?? currentAmountEdits) }
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
      if (!newIncomeSourceId) return
      const source = incomeSourceById.get(Number(newIncomeSourceId))
      createIncome.mutate({ sourceId: Number(newIncomeSourceId), label: source?.name, amountCents: parseCents(newAmount), notes: '', sortOrder: incomes.length }, {
        onSuccess: () => { setNewIncomeSourceId(null); setNewAmount(''); setIncomeSheet(null) }
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
              {isAdmin && (isClosed
                ? <Menu.Item color="orange" onClick={handleReopen}>{t('month.reopenAction')}</Menu.Item>
                : <Menu.Item color="green" onClick={handleClose}>{t('month.closeAction')}</Menu.Item>)}
              {isAdmin && !isClosed && <Menu.Item color="red" onClick={() => setDeleteOpen(true)}>{t('month.delete')}</Menu.Item>}
            </Menu.Dropdown>
          </Menu>
        </Group>
        <Title order={3}>{monthName} {y}</Title>

        <HeroStat label={t('month.surplusLabel')} value={<MoneyText cents={surplusCents} span fw={800} size="2.5rem" colored />} />

        <Stack gap="xs">
          <Group justify="space-between">
            <Text fw={600} size="sm">{t('month.income')}</Text>
            <SortControl mode={incomeSort} onChange={setIncomeSort} />
          </Group>
          <MobileList>
            {sortedIncomes.map(inc => (
              <MobileListRow
                key={inc.id}
                title={inc.label ?? t('month.unknownSource', { id: inc.sourceId })}
                subtitle={inc.entryType === 'carryover' ? t('month.carryoverBadge') : undefined}
                trailing={<MoneyText cents={inc.effectiveCents} fw={600} />}
                to={inc.isItemized ? `/months/${y}/${m}/income?source=${inc.sourceId}` : undefined}
                chevron={inc.isItemized || (!isClosed && inc.entryType !== 'carryover')}
                onClick={!inc.isItemized && !isClosed && inc.entryType !== 'carryover' ? () => { setIncomeSheet(inc.id); setEditIncomeAmount(inc.amountCents / 100) } : undefined}
              />
            ))}
            {!isClosed && (
              <MobileListRow
                title={t('month.addIncome')}
                leftSection={<IconPlus size={16} />}
                onClick={() => { setNewIncomeSourceId(null); setNewAmount(''); setIncomeSheet('create') }}
              />
            )}
          </MobileList>
          <Group justify="space-between" px="xs">
            <Text fw={700} size="sm">{t('month.totalIncome')}</Text>
            <MoneyText cents={incomeTotalCents} fw={700} />
          </Group>
        </Stack>

        <Stack gap="xs">
          <Group justify="space-between">
            <Text fw={600} size="sm">{t('month.expenses')}</Text>
            <SortControl mode={expenseSort} onChange={setExpenseSort} />
          </Group>
          <MobileList>
            {sortedBudgetLines.map(bl => {
              const budgeted = bl.targetCents > 0
              const pct = budgeted ? (bl.effectiveCents / bl.targetCents) * 100 : 0
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
                        <>
                          {!bl.tracksTransactions && (
                            <ActionIcon
                              size="sm"
                              variant="subtle"
                              aria-label={t('common.edit')}
                              onClick={(e) => { e.preventDefault(); e.stopPropagation(); openEditBudgetAmount(bl) }}
                            ><IconPencil size={14} /></ActionIcon>
                          )}
                          <Menu position="bottom-end" withinPortal>
                            <Menu.Target>
                              <ActionIcon
                                size="sm"
                                variant={bl.tracksTransactions ? 'filled' : 'subtle'}
                                color={bl.tracksTransactions ? 'blue' : 'gray'}
                                aria-label={t('month.moreOptions')}
                                onClick={(e) => { e.preventDefault(); e.stopPropagation() }}
                              >≡</ActionIcon>
                            </Menu.Target>
                            <Menu.Dropdown onClick={(e) => { e.preventDefault(); e.stopPropagation() }}>
                              <Menu.Item onClick={() => handleToggleTracking(bl)}>
                                {bl.tracksTransactions ? t('month.trackTransactionsDisable') : t('month.trackTransactionsEnable')}
                              </Menu.Item>
                              <Menu.Item
                                color="red"
                                leftSection={<IconTrash size={14} />}
                                disabled={bl.effectiveCents !== 0 || bl.transactionsTotalCents !== 0}
                                onClick={() => handleDeleteBudgetLine(bl, budgetLineLabel(bl))}
                              >
                                {bl.effectiveCents !== 0 || bl.transactionsTotalCents !== 0 ? t('month.deleteBudgetLineDisabledHint') : t('common.delete')}
                              </Menu.Item>
                            </Menu.Dropdown>
                          </Menu>
                        </>
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
            <Group gap="xs">
              <SortControl mode={splitSort} onChange={setSplitSort} />
              {!isClosed && !splitEdits && (
                <Button size="xs" variant="subtle" onClick={startSplitEdit}>
                  {t('month.adjust')}
                </Button>
              )}
            </Group>
          </Group>
          <MobileList>
            {sortedSplitPotIds.map(potId => {
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
              <Select
                label={t('settings.incomeSources')}
                placeholder={t('month.addIncome')}
                data={availableIncomeSources.map(s => ({ value: String(s.id), label: s.name }))}
                value={newIncomeSourceId}
                onChange={setNewIncomeSourceId}
                searchable
                clearable
              />
              <NumberInput label={t('common.amount')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls placeholder="0,00" />
              <Button disabled={!newIncomeSourceId} loading={createIncome.isPending} onClick={handleCreateIncome}>{t('common.add')}</Button>
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
                  disabled={editingIncome.effectiveCents !== 0 || editingIncome.transactionsTotalCents !== 0}
                  title={editingIncome.effectiveCents !== 0 || editingIncome.transactionsTotalCents !== 0 ? t('month.deleteIncomeDisabledHint') : undefined}
                  onClick={() => handleDeleteIncome(editingIncome, editingIncome.label || t('month.unknownSource', { id: editingIncome.sourceId }), () => setIncomeSheet(null))}
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

        <BottomSheet opened={!!splitEdits} onClose={cancelSplitEdit} title={t('month.splitSection')}>
          {carryoverOverAllocated && <Text size="xs" c="red">{t('month.splitOverAllocated')}</Text>}
          {amountSplitDisabled && <Text size="xs" c="dimmed">{t('month.splitAmountNeedsSurplus')}</Text>}
          <Stack gap="sm">
            {splitPotIds.map(potId => {
              const sp = splits.find(s => s.potId === potId)
              const name = sp?.potName ?? potNameById.get(potId) ?? '—'
              const isCarryoverRow = !!carryoverPot && potId === carryoverPot.id
              return (
                <Group key={potId} justify="space-between" wrap="nowrap">
                  <Text size="sm" style={{ flex: 1 }}>{name}{isCarryoverRow && <Text span size="xs" c="dimmed"> ({t('pots.kind_carryover')})</Text>}</Text>
                  {isCarryoverRow ? (
                    <Group gap={6} wrap="nowrap">
                      <Text size="sm" c={carryoverOverAllocated ? 'red' : 'dimmed'}>{carryoverRemainder.toFixed(2)}%</Text>
                      <MoneyText cents={pctToProjectedCents(carryoverRemainder)} size="sm" c="dimmed" />
                    </Group>
                  ) : (
                    <Group gap={6} wrap="nowrap">
                      <NumberInput
                        size="sm"
                        value={splitEdits?.[potId] ?? sp?.percentage ?? ''}
                        onChange={(v) => setSplitPercentage(potId, String(v))}
                        suffix="%"
                        decimalScale={2}
                        hideControls
                        min={0}
                        max={100}
                        w={90}
                      />
                      <NumberInput
                        size="sm"
                        value={currentAmountEdits[potId] ?? '0'}
                        onChange={(v) => setSplitAmountText(potId, String(v), Number(v) || 0)}
                        decimalSeparator=","
                        decimalScale={2}
                        prefix="€ "
                        hideControls
                        disabled={amountSplitDisabled}
                        w={100}
                      />
                    </Group>
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
              <Button variant="subtle" onClick={cancelSplitEdit}>{t('common.cancel')}</Button>
            </Group>
          </Stack>
        </BottomSheet>

        <Modal opened={editBudgetLineId !== null} onClose={() => setEditBudgetLineId(null)} title={t('month.editActualAmount')}>
          <Stack gap="sm">
            <NumberInput
              label={t('common.amount')}
              value={editBudgetAmount}
              onChange={setEditBudgetAmount}
              decimalSeparator=","
              decimalScale={2}
              prefix="€ "
              hideControls
              data-autofocus
            />
            <Group grow>
              <Button onClick={handleSaveBudgetAmount} loading={updateBudgetLine.isPending}>{t('common.save')}</Button>
              <Button variant="subtle" onClick={() => setEditBudgetLineId(null)}>{t('common.cancel')}</Button>
            </Group>
          </Stack>
        </Modal>

        <ReauthConfirmModal
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
          {isAdmin && !isClosed && (
            <Button color="red" variant="subtle" size="sm" onClick={() => setDeleteOpen(true)}>
              {t('month.delete')}
            </Button>
          )}
          {isAdmin && (isClosed
            ? <Button color="orange" onClick={handleReopen} loading={reopenPeriod.isPending}>{t('month.reopenAction')}</Button>
            : <Button color="green" onClick={handleClose} loading={closePeriod.isPending}>{t('month.closeAction')}</Button>
          )}
        </Group>
      </Group>

      {/* Incomes */}
      <Paper shadow="xs" p="md" withBorder>
        <Group justify="space-between" mb="sm">
          <Title order={4}>{t('month.income')}</Title>
          <SortControl mode={incomeSort} onChange={setIncomeSort} />
        </Group>
        <Table.ScrollContainer minWidth={420}>
        <Table>
          <Table.Tbody>
            {sortedIncomes.map(inc => (
              <Table.Tr key={inc.id}>
                <Table.Td>
                  {inc.isItemized
                    ? <Text component={Link} to={`/months/${y}/${m}/income?source=${inc.sourceId}`} c="blue" size="sm">{inc.label ?? t('month.unknownSource', { id: inc.sourceId })}</Text>
                    : <Text size="sm" c={inc.entryType === 'carryover' ? 'dimmed' : undefined}>
                        {inc.label ?? t('month.unknownSource', { id: inc.sourceId })}
                        {inc.entryType === 'carryover' && <Badge size="xs" ml="xs" color="gray">{t('month.carryoverBadge')}</Badge>}
                      </Text>
                  }
                </Table.Td>
                <Table.Td ta="right" w={160}>
                  {inc.isItemized || isClosed || inc.entryType === 'carryover'
                    ? <MoneyText cents={inc.effectiveCents} />
                    : <NumberInput
                        // Remounts when the server value changes so a failed PUT
                        // (or another session's edit) can't leave the locally
                        // typed number on screen as if it had been saved.
                        key={inc.amountCents}
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
                {!isClosed && !inc.isItemized && inc.entryType !== 'carryover' && (
                  <Table.Td w={40}>
                    <ActionIcon
                      color="red"
                      size="sm"
                      variant="subtle"
                      disabled={inc.effectiveCents !== 0 || inc.transactionsTotalCents !== 0}
                      title={inc.effectiveCents !== 0 || inc.transactionsTotalCents !== 0 ? t('month.deleteIncomeDisabledHint') : t('common.delete')}
                      aria-label={t('common.delete')}
                      onClick={() => handleDeleteIncome(inc, inc.label || t('month.unknownSource', { id: inc.sourceId }))}
                    ><IconTrash size={14} /></ActionIcon>
                  </Table.Td>
                )}
              </Table.Tr>
            ))}
          </Table.Tbody>
          <Table.Tfoot>
            {!isClosed && (
              <Table.Tr>
                <Table.Td>
                  <Select
                    size="xs"
                    placeholder={t('settings.incomeSources')}
                    data={availableIncomeSources.map(s => ({ value: String(s.id), label: s.name }))}
                    value={newIncomeSourceId}
                    onChange={setNewIncomeSourceId}
                    searchable
                    clearable
                  />
                </Table.Td>
                <Table.Td>
                  <NumberInput size="xs" value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls placeholder="0,00" />
                </Table.Td>
                <Table.Td>
                  <Button size="xs" aria-label={t('common.add')} disabled={!newIncomeSourceId} loading={createIncome.isPending} onClick={() => {
                    const source = incomeSourceById.get(Number(newIncomeSourceId))
                    createIncome.mutate({ sourceId: Number(newIncomeSourceId), label: source?.name, amountCents: parseCents(newAmount), notes: '', sortOrder: incomes.length }, {
                      onSuccess: () => { setNewIncomeSourceId(null); setNewAmount('') }
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
        <Group justify="space-between" mb="sm">
          <Title order={4}>{t('month.expenses')}</Title>
          <SortControl mode={expenseSort} onChange={setExpenseSort} />
        </Group>
        <Table.ScrollContainer minWidth={420}>
        <Table>
          <Table.Tbody>
            {sortedBudgetLines.map(bl => {
              const budgeted = bl.targetCents > 0
              const pct = budgeted ? (bl.effectiveCents / bl.targetCents) * 100 : 0
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
                  <Group gap={4} justify="flex-end" wrap="nowrap">
                    <MoneyText cents={bl.effectiveCents} />
                    {bl.targetCents > 0 && (
                      <Text size="xs" c="dimmed" span>/ {(bl.targetCents / 100).toFixed(2)}</Text>
                    )}
                  </Group>
                  {budgeted && (
                    <Stack gap={2} mt={4} align="flex-end">
                      <Progress value={Math.min(pct, 100)} color={progressColor} size="sm" w="100%" />
                      <Text size="xs" c="dimmed">
                        {pct.toFixed(0)}% — {t('month.remaining')}: <MoneyText cents={bl.targetCents - bl.effectiveCents} colored span size="xs" />
                      </Text>
                    </Stack>
                  )}
                </Table.Td>
                {!isClosed && (
                  <Table.Td w={60}>
                    <Group gap={4} wrap="nowrap">
                      {!bl.tracksTransactions && (
                        <ActionIcon
                          size="xs"
                          variant="subtle"
                          title={t('common.edit')}
                          aria-label={t('common.edit')}
                          onClick={() => openEditBudgetAmount(bl)}
                        ><IconPencil size={14} /></ActionIcon>
                      )}
                      <Menu position="bottom-end" withinPortal>
                        <Menu.Target>
                          <ActionIcon
                            size="xs"
                            variant={bl.tracksTransactions ? 'filled' : 'subtle'}
                            color={bl.tracksTransactions ? 'blue' : 'gray'}
                            title={t('month.moreOptions')}
                            aria-label={t('month.moreOptions')}
                          >≡</ActionIcon>
                        </Menu.Target>
                        <Menu.Dropdown>
                          <Menu.Item onClick={() => handleToggleTracking(bl)}>
                            {bl.tracksTransactions ? t('month.trackTransactionsDisable') : t('month.trackTransactionsEnable')}
                          </Menu.Item>
                          <Menu.Item
                            color="red"
                            leftSection={<IconTrash size={14} />}
                            disabled={bl.effectiveCents !== 0 || bl.transactionsTotalCents !== 0}
                            onClick={() => handleDeleteBudgetLine(bl, budgetLineLabel(bl))}
                          >
                            {bl.effectiveCents !== 0 || bl.transactionsTotalCents !== 0 ? t('month.deleteBudgetLineDisabledHint') : t('common.delete')}
                          </Menu.Item>
                        </Menu.Dropdown>
                      </Menu>
                    </Group>
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
          <Group gap="xs">
            <SortControl mode={splitSort} onChange={setSplitSort} />
            {!isClosed && (
              splitEdits
                ? <Group gap="xs">
                    <Button size="xs" onClick={handleSaveSplits} loading={replaceSplits.isPending} disabled={carryoverOverAllocated}>{t('common.save')}</Button>
                    <Button size="xs" variant="subtle" onClick={cancelSplitEdit}>{t('common.cancel')}</Button>
                  </Group>
                : <Button size="xs" variant="subtle" onClick={startSplitEdit}>
                    {t('month.adjust')}
                  </Button>
            )}
          </Group>
        </Group>
        {carryoverOverAllocated && (
          <Text size="xs" c="red" mb="sm">{t('month.splitOverAllocated')}</Text>
        )}
        {splitEdits && amountSplitDisabled && (
          <Text size="xs" c="dimmed" mb="sm">{t('month.splitAmountNeedsSurplus')}</Text>
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
            {sortedSplitPotIds.map(potId => {
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
                            value={splitEdits[potId] ?? sp?.percentage ?? ''}
                            onChange={(v) => setSplitPercentage(potId, String(v))}
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
                <Table.Td ta="right" w={130}>
                  {splitEdits
                    ? (isCarryoverRow
                        ? <MoneyText cents={pctToProjectedCents(carryoverRemainder)} colored={!isClosed} />
                        : <NumberInput
                            size="xs"
                            value={currentAmountEdits[potId] ?? '0'}
                            onChange={(v) => setSplitAmountText(potId, String(v), Number(v) || 0)}
                            decimalSeparator=","
                            decimalScale={2}
                            prefix="€ "
                            hideControls
                            disabled={amountSplitDisabled}
                          />
                      )
                    : (sp ? <MoneyText cents={sp.projectedCents ?? 0} colored={!isClosed} /> : <Text size="sm" c="dimmed">—</Text>)
                  }
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
      <Modal opened={editBudgetLineId !== null} onClose={() => setEditBudgetLineId(null)} title={t('month.editActualAmount')}>
        <Stack gap="sm">
          <NumberInput
            label={t('common.amount')}
            value={editBudgetAmount}
            onChange={setEditBudgetAmount}
            decimalSeparator=","
            decimalScale={2}
            prefix="€ "
            hideControls
            data-autofocus
          />
          <Group grow>
            <Button onClick={handleSaveBudgetAmount} loading={updateBudgetLine.isPending}>{t('common.save')}</Button>
            <Button variant="subtle" onClick={() => setEditBudgetLineId(null)}>{t('common.cancel')}</Button>
          </Group>
        </Stack>
      </Modal>

      <ReauthConfirmModal
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

  function handleDelete() {
    deletePeriod.mutate(undefined, {
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
