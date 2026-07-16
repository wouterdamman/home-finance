import { useMemo, useState } from 'react'
import { Title, Tabs, Table, Button, Group, TextInput, NumberInput, Switch, Stack, Badge, Select, Tooltip, Text, Modal } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { DateInput } from '@mantine/dates'
import { IconX, IconSearch, IconPlus, IconChevronUp, IconChevronDown, IconArrowsSort } from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import {
  useCategories, useCreateCategory, useUpdateCategory, useArchiveCategory,
  useIncomeSources, useCreateIncomeSource, useUpdateIncomeSource, useArchiveIncomeSource,
  usePots, useCreatePot, useUpdatePot, useArchivePot,
} from '../api/hooks/useSettings'
import { useYears, useCreateYear } from '../api/hooks/usePeriods'
import { parseToCents } from '../lib/money'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'

function amountToCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

type SortDir = 'asc' | 'desc'
interface SortState<K extends string> { key: K; dir: SortDir }

function toggleSort<K extends string>(sort: SortState<K>, key: K): SortState<K> {
  if (sort.key !== key) return { key, dir: 'asc' }
  return { key, dir: sort.dir === 'asc' ? 'desc' : 'asc' }
}

function SortableTh<K extends string>({ label, sortKey, sort, onSort, ta }: {
  label: React.ReactNode
  sortKey: K
  sort: SortState<K>
  onSort: (key: K) => void
  ta?: 'left' | 'right' | 'center'
}) {
  const active = sort.key === sortKey
  const Icon = active ? (sort.dir === 'asc' ? IconChevronUp : IconChevronDown) : IconArrowsSort
  return (
    <Table.Th ta={ta} onClick={() => onSort(sortKey)} style={{ cursor: 'pointer', userSelect: 'none', whiteSpace: 'nowrap' }}>
      <Group gap={4} wrap="nowrap" justify={ta === 'right' ? 'flex-end' : ta === 'center' ? 'center' : 'flex-start'}>
        {label}
        <Icon size={13} opacity={active ? 1 : 0.4} />
      </Group>
    </Table.Th>
  )
}

function cmp(a: number | string, b: number | string): number {
  if (typeof a === 'string' && typeof b === 'string') return a.localeCompare(b)
  return (a as number) - (b as number)
}

export default function Settings() {
  const { t } = useTranslation()
  return (
    <>
      <Title order={2} mb="md">{t('settings.title')}</Title>
      <Tabs defaultValue="categories">
        <Tabs.List>
          <Tabs.Tab value="categories">{t('settings.categories')}</Tabs.Tab>
          <Tabs.Tab value="sources">{t('settings.incomeSources')}</Tabs.Tab>
          <Tabs.Tab value="pots">{t('settings.pots')}</Tabs.Tab>
          <Tabs.Tab value="years">{t('settings.years')}</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="categories" pt="md"><CategoriesTab /></Tabs.Panel>
        <Tabs.Panel value="sources" pt="md"><SourcesTab /></Tabs.Panel>
        <Tabs.Panel value="pots" pt="md"><PotsTab /></Tabs.Panel>
        <Tabs.Panel value="years" pt="md"><YearsTab /></Tabs.Panel>
      </Tabs>
    </>
  )
}

type CategorySortKey = 'name' | 'default' | 'itemized' | 'template'

function CategoriesTab() {
  const { t } = useTranslation()
  const { data } = useCategories()
  const create = useCreateCategory()
  const update = useUpdateCategory()
  const archive = useArchiveCategory()
  const [editing, setEditing] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [editItemized, setEditItemized] = useState(false)
  const [editTemplate, setEditTemplate] = useState(true)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newItemized, setNewItemized] = useState(false)
  const [newTemplate, setNewTemplate] = useState(true)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortState<CategorySortKey>>({ key: 'name', dir: 'asc' })
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  const startEdit = (cat: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean }) => {
    setEditing(cat.id)
    setEditName(cat.name)
    setEditAmount(cat.defaultAmountCents / 100)
    setEditItemized(cat.isItemized)
    setEditTemplate(cat.includeInTemplate)
  }

  const saveEdit = () => {
    if (!editing) return
    update.mutate({ id: editing, name: editName, defaultAmountCents: amountToCents(editAmount), isItemized: editItemized, includeInTemplate: editTemplate }, {
      onSuccess: () => setEditing(null)
    })
  }

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(c => c.name.toLowerCase().includes(search.toLowerCase()))
    return filtered.sort((a, b) => {
      const c = sort.key === 'name' ? cmp(a.name, b.name)
        : sort.key === 'default' ? cmp(a.defaultAmountCents, b.defaultAmountCents)
        : sort.key === 'itemized' ? cmp(Number(a.isItemized), Number(b.isItemized))
        : cmp(Number(a.includeInTemplate), Number(b.includeInTemplate))
      return sort.dir === 'asc' ? c : -c
    })
  }, [data, search, sort])

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
          w={260}
        />
        <Button leftSection={<IconPlus size={14} />} onClick={openAdd}>{t('common.add')}</Button>
      </Group>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <SortableTh label={t('settings.name')} sortKey="name" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} />
            <SortableTh label={t('settings.default')} sortKey="default" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} ta="right" />
            <SortableTh label={t('settings.itemized')} sortKey="itemized" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} ta="center" />
            <SortableTh
              label={<Tooltip label={t('settings.templateHint')}><span>{t('settings.template')}</span></Tooltip>}
              sortKey="template" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} ta="center"
            />
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.length === 0 && (
            <Table.Tr><Table.Td colSpan={5}><EmptyState message={t('settings.noCategories')} /></Table.Td></Table.Tr>
          )}
          {rows.map(cat => (
            <Table.Tr key={cat.id} opacity={cat.archivedAt ? 0.5 : 1}>
              {editing === cat.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Table.Td>
                    <Table.Td ta="center"><Switch checked={editItemized} onChange={e => setEditItemized(e.target.checked)} /></Table.Td>
                    <Table.Td ta="center"><Switch checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} /></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={saveEdit}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}><IconX size={14} /></Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{cat.name} {cat.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
                    <Table.Td ta="right">€ {(cat.defaultAmountCents / 100).toFixed(2)}</Table.Td>
                    <Table.Td ta="center">{cat.isItemized ? '✓' : ''}</Table.Td>
                    <Table.Td ta="center">{cat.includeInTemplate ? '✓' : '—'}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => startEdit(cat)}>{t('common.edit')}</Button>
                        <Button size="xs" variant="subtle" color={cat.archivedAt ? 'green' : 'red'} onClick={() => archive.mutate(cat.id)}>
                          {cat.archivedAt ? t('settings.restore') : t('common.archive')}
                        </Button>
                      </Group>
                    </Table.Td>
                  </>
              }
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Modal opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
        <Stack gap="sm">
          <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
          <NumberInput label={t('settings.default')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
          <Switch label={t('settings.itemized')} checked={newItemized} onChange={e => setNewItemized(e.target.checked)} />
          <Switch label={t('settings.template')} checked={newTemplate} onChange={e => setNewTemplate(e.target.checked)} />
          <Button disabled={!newName} loading={create.isPending} onClick={() => {
            create.mutate({ name: newName, defaultAmountCents: amountToCents(newAmount), isItemized: newItemized, includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0) }, {
              onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false); setNewTemplate(true); closeAdd() }
            })
          }}>{t('common.add')}</Button>
        </Stack>
      </Modal>
    </Stack>
  )
}

type SourceSortKey = 'name' | 'default' | 'template'

function SourcesTab() {
  const { t } = useTranslation()
  const { data } = useIncomeSources()
  const create = useCreateIncomeSource()
  const update = useUpdateIncomeSource()
  const archive = useArchiveIncomeSource()
  const [editing, setEditing] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [editTemplate, setEditTemplate] = useState(true)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newTemplate, setNewTemplate] = useState(true)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortState<SourceSortKey>>({ key: 'name', dir: 'asc' })
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(s => s.name.toLowerCase().includes(search.toLowerCase()))
    return filtered.sort((a, b) => {
      const c = sort.key === 'name' ? cmp(a.name, b.name)
        : sort.key === 'default' ? cmp(a.defaultAmountCents, b.defaultAmountCents)
        : cmp(Number(a.includeInTemplate), Number(b.includeInTemplate))
      return sort.dir === 'asc' ? c : -c
    })
  }, [data, search, sort])

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
          w={260}
        />
        <Button leftSection={<IconPlus size={14} />} onClick={openAdd}>{t('common.add')}</Button>
      </Group>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <SortableTh label={t('settings.name')} sortKey="name" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} />
            <SortableTh label={t('settings.default')} sortKey="default" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} ta="right" />
            <SortableTh
              label={<Tooltip label={t('settings.templateHint')}><span>{t('settings.template')}</span></Tooltip>}
              sortKey="template" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} ta="center"
            />
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.length === 0 && (
            <Table.Tr><Table.Td colSpan={4}><EmptyState message={t('settings.noSources')} /></Table.Td></Table.Tr>
          )}
          {rows.map(src => (
            <Table.Tr key={src.id} opacity={src.archivedAt ? 0.5 : 1}>
              {editing === src.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Table.Td>
                    <Table.Td ta="center"><Switch checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} /></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={() => update.mutate({ id: src.id, name: editName, defaultAmountCents: amountToCents(editAmount), includeInTemplate: editTemplate, sortOrder: src.sortOrder }, { onSuccess: () => setEditing(null) })}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}><IconX size={14} /></Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{src.name} {src.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
                    <Table.Td ta="right">€ {(src.defaultAmountCents / 100).toFixed(2)}</Table.Td>
                    <Table.Td ta="center">{src.includeInTemplate ? '✓' : '—'}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => { setEditing(src.id); setEditName(src.name); setEditAmount(src.defaultAmountCents / 100); setEditTemplate(src.includeInTemplate) }}>{t('common.edit')}</Button>
                        <Button size="xs" variant="subtle" color={src.archivedAt ? 'green' : 'red'} onClick={() => archive.mutate(src.id)}>
                          {src.archivedAt ? t('settings.restore') : t('common.archive')}
                        </Button>
                      </Group>
                    </Table.Td>
                  </>
              }
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Modal opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
        <Stack gap="sm">
          <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
          <NumberInput label={t('settings.default')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
          <Switch label={t('settings.template')} checked={newTemplate} onChange={e => setNewTemplate(e.target.checked)} />
          <Button disabled={!newName} loading={create.isPending} onClick={() => {
            create.mutate({ name: newName, defaultAmountCents: amountToCents(newAmount), includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0) }, {
              onSuccess: () => { setNewName(''); setNewAmount(''); setNewTemplate(true); closeAdd() }
            })
          }}>{t('common.add')}</Button>
        </Stack>
      </Modal>
    </Stack>
  )
}

type PotSortKey = 'name' | 'kind' | 'target'

function PotsTab() {
  const { t } = useTranslation()
  const { data } = usePots()
  const create = useCreatePot()
  const update = useUpdatePot()
  const archive = useArchivePot()
  const [editing, setEditing] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editKind, setEditKind] = useState('normal')
  const [editTarget, setEditTarget] = useState<number | string>('')
  const [editTargetDate, setEditTargetDate] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newKind, setNewKind] = useState<string>('normal')
  const [newTarget, setNewTarget] = useState<number | string>('')
  const [newTargetDate, setNewTargetDate] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortState<PotSortKey>>({ key: 'name', dir: 'asc' })
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  const kindLabel = (kind: string) =>
    kind === 'carryover' ? t('settings.kind_carryover') : t('settings.kind_normal')

  const handleCreate = () => {
    create.mutate({
      name: newName,
      kind: newKind,
      sortOrder: (data?.length ?? 0),
      targetCents: newKind === 'normal' && newTarget !== '' ? amountToCents(newTarget) : null,
      targetDate: newKind === 'normal' ? newTargetDate : null,
    }, {
      onSuccess: () => { setNewName(''); setNewKind('normal'); setNewTarget(''); setNewTargetDate(null); closeAdd() },
      onError: (err: unknown) => {
        const msg = (err as { body?: { error?: { message?: string } } })?.body?.error?.message ?? String(err)
        notifications.show({
          color: 'red',
          title: t('common.error'),
          message: newKind === 'carryover'
            ? t('settings.carryoverExists')
            : msg,
        })
      },
    })
  }

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(p => p.name.toLowerCase().includes(search.toLowerCase()))
    return filtered.sort((a, b) => {
      const c = sort.key === 'name' ? cmp(a.name, b.name)
        : sort.key === 'kind' ? cmp(a.kind, b.kind)
        : cmp(a.targetCents ?? -1, b.targetCents ?? -1)
      return sort.dir === 'asc' ? c : -c
    })
  }, [data, search, sort])

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
          w={260}
        />
        <Button leftSection={<IconPlus size={14} />} onClick={openAdd}>{t('common.add')}</Button>
      </Group>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <SortableTh label={t('settings.name')} sortKey="name" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} />
            <SortableTh label={t('settings.type')} sortKey="kind" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} />
            <SortableTh label={t('pots.target')} sortKey="target" sort={sort} onSort={k => setSort(s => toggleSort(s, k))} />
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.length === 0 && (
            <Table.Tr><Table.Td colSpan={4}><EmptyState message={t('settings.noPots')} /></Table.Td></Table.Tr>
          )}
          {rows.map(pot => (
            <Table.Tr key={pot.id} opacity={pot.archivedAt ? 0.5 : 1}>
              {editing === pot.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td>
                      <Select
                        size="xs"
                        w={130}
                        value={editKind}
                        onChange={v => setEditKind(v ?? 'normal')}
                        data={[
                          { value: 'normal', label: t('settings.kind_normal') },
                          { value: 'carryover', label: t('settings.kind_carryover') },
                        ]}
                      />
                    </Table.Td>
                    <Table.Td>
                      {editKind === 'normal' && (
                        <Group gap="xs" wrap="nowrap">
                          <NumberInput
                            size="xs"
                            w={110}
                            value={editTarget}
                            onChange={setEditTarget}
                            decimalSeparator=","
                            decimalScale={2}
                            prefix="€ "
                            hideControls
                            placeholder="0,00"
                          />
                          <DateInput
                            size="xs"
                            w={130}
                            value={editTargetDate}
                            onChange={setEditTargetDate}
                            valueFormat="DD-MM-YYYY"
                            clearable
                          />
                        </Group>
                      )}
                    </Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={() => update.mutate({
                          id: pot.id,
                          name: editName,
                          kind: editKind,
                          sortOrder: pot.sortOrder,
                          targetCents: editKind === 'normal' && editTarget !== '' ? amountToCents(editTarget) : null,
                          targetDate: editKind === 'normal' ? editTargetDate : null,
                        }, {
                          onSuccess: () => setEditing(null),
                          onError: (err: unknown) => {
                            const msg = (err as { body?: { error?: { message?: string } } })?.body?.error?.message ?? String(err)
                            notifications.show({ color: 'red', title: t('common.error'), message: editKind === 'carryover' ? t('settings.carryoverExists') : msg })
                          },
                        })}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}><IconX size={14} /></Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{pot.name} {pot.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
                    <Table.Td><Badge size="xs" color={pot.kind === 'carryover' ? 'blue' : 'gray'}>{kindLabel(pot.kind)}</Badge></Table.Td>
                    <Table.Td>
                      {pot.targetCents != null && (
                        <MoneyText cents={pot.targetCents} size="sm" />
                      )}
                      {pot.targetDate && <Text size="xs" c="dimmed">{dayjs(pot.targetDate).format('DD-MM-YYYY')}</Text>}
                    </Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => {
                          setEditing(pot.id)
                          setEditName(pot.name)
                          setEditKind(pot.kind)
                          setEditTarget(pot.targetCents != null ? pot.targetCents / 100 : '')
                          setEditTargetDate(pot.targetDate ?? null)
                        }}>{t('common.edit')}</Button>
                        <Button size="xs" variant="subtle" color={pot.archivedAt ? 'green' : 'red'} onClick={() => archive.mutate(pot.id)}>
                          {pot.archivedAt ? t('settings.restore') : t('common.archive')}
                        </Button>
                      </Group>
                    </Table.Td>
                  </>
              }
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Modal opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
        <Stack gap="sm">
          <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
          <Select
            label={t('settings.type')}
            value={newKind}
            onChange={v => setNewKind(v ?? 'normal')}
            data={[
              { value: 'normal', label: t('settings.kind_normal') },
              { value: 'carryover', label: t('settings.kind_carryover') },
            ]}
          />
          {newKind === 'normal' && (
            <>
              <NumberInput
                label={t('pots.target')}
                value={newTarget}
                onChange={setNewTarget}
                decimalSeparator=","
                decimalScale={2}
                prefix="€ "
                hideControls
              />
              <DateInput
                label={t('pots.targetDate')}
                value={newTargetDate}
                onChange={setNewTargetDate}
                valueFormat="DD-MM-YYYY"
                clearable
              />
            </>
          )}
          <Button disabled={!newName} loading={create.isPending} onClick={handleCreate}>
            {t('common.add')}
          </Button>
        </Stack>
      </Modal>
    </Stack>
  )
}

function YearsTab() {
  const { t } = useTranslation()
  const { data } = useYears()
  const create = useCreateYear()
  const [newYear, setNewYear] = useState<number | string>('')
  const [search, setSearch] = useState('')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(y => String(y).includes(search))
    return [...filtered].sort((a, b) => sortDir === 'asc' ? a - b : b - a)
  }, [data, search, sortDir])

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
          w={260}
        />
        <Button leftSection={<IconPlus size={14} />} onClick={openAdd}>{t('common.add')}</Button>
      </Group>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th onClick={() => setSortDir(d => d === 'asc' ? 'desc' : 'asc')} style={{ cursor: 'pointer', userSelect: 'none', whiteSpace: 'nowrap' }}>
              <Group gap={4} wrap="nowrap">
                {t('settings.year')}
                {sortDir === 'asc' ? <IconChevronUp size={13} /> : <IconChevronDown size={13} />}
              </Group>
            </Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.length === 0 && (
            <Table.Tr><Table.Td><EmptyState message={t('settings.noYears')} /></Table.Td></Table.Tr>
          )}
          {rows.map(y => (
            <Table.Tr key={y}>
              <Table.Td>{y}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Modal opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
        <Stack gap="sm">
          <NumberInput label={t('settings.year')} value={newYear} onChange={setNewYear} hideControls decimalScale={0} />
          <Button disabled={!newYear} loading={create.isPending} onClick={() => {
            create.mutate(Number(newYear), { onSuccess: () => { setNewYear(''); closeAdd() } })
          }}>{t('common.add')}</Button>
        </Stack>
      </Modal>
    </Stack>
  )
}
