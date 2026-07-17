import { useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Title, Tabs, Table, Button, Group, TextInput, NumberInput, Switch, Stack, Badge, Select, MultiSelect, Tooltip, Text, Modal, Menu, ActionIcon, SegmentedControl, Divider, FileInput, useMantineColorScheme } from '@mantine/core'
import { useDisclosure, useMediaQuery } from '@mantine/hooks'
import { DateInput } from '@mantine/dates'
import {
  IconX, IconSearch, IconPlus, IconChevronUp, IconChevronDown, IconArrowsSort,
  IconTags, IconCoin, IconPigMoney, IconCalendar, IconChevronLeft, IconPalette,
  IconSun, IconMoon, IconDeviceDesktop, IconFileSpreadsheet, IconDownload, IconUpload,
} from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import {
  useCategories, useCreateCategory, useUpdateCategory, useArchiveCategory,
  useIncomeSources, useCreateIncomeSource, useUpdateIncomeSource, useArchiveIncomeSource,
  usePots, useCreatePot, useUpdatePot, useArchivePot,
} from '../api/hooks/useSettings'
import { useYears, useCreateYear, useImportXLSX } from '../api/hooks/usePeriods'
import { useCurrentYear } from '../api/hooks/useCurrentYear'
import { parseToCents } from '../lib/money'
import { getErrorMessage } from '../api/client'
import type { ImportReport } from '../api/types'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'
import PasswordModal from '../components/PasswordModal'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import BottomSheet from '../components/mobile/BottomSheet'

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

function SortMenu<K extends string>({ sort, onSort, options }: {
  sort: SortState<K>
  onSort: (key: K) => void
  options: { key: K; label: string }[]
}) {
  const { t } = useTranslation()
  const activeLabel = options.find(o => o.key === sort.key)?.label ?? ''
  return (
    <Menu position="bottom-end">
      <Menu.Target>
        <Button size="xs" variant="subtle" rightSection={sort.dir === 'asc' ? <IconChevronUp size={13} /> : <IconChevronDown size={13} />}>
          {t('common.sortBy')}: {activeLabel}
        </Button>
      </Menu.Target>
      <Menu.Dropdown>
        {options.map(o => (
          <Menu.Item key={o.key} onClick={() => onSort(o.key)} fw={sort.key === o.key ? 700 : 400}>
            {o.label}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  )
}

type SettingsSection = 'categories' | 'sources' | 'pots' | 'years' | 'exportImport' | 'preferences'

export default function Settings() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const [searchParams, setSearchParams] = useSearchParams()
  const section = searchParams.get('section') as SettingsSection | null

  if (isMobile) {
    const menu: { key: SettingsSection; label: string; icon: typeof IconTags; content: React.ReactNode }[] = [
      { key: 'categories', label: t('settings.categories'), icon: IconTags, content: <CategoriesTab /> },
      { key: 'sources', label: t('settings.incomeSources'), icon: IconCoin, content: <SourcesTab /> },
      { key: 'pots', label: t('settings.pots'), icon: IconPigMoney, content: <PotsTab /> },
      { key: 'years', label: t('settings.years'), icon: IconCalendar, content: <YearsTab /> },
      { key: 'exportImport', label: t('settings.exportImport'), icon: IconFileSpreadsheet, content: <ExportImportTab /> },
      { key: 'preferences', label: t('settings.preferences'), icon: IconPalette, content: <PreferencesTab /> },
    ]
    const active = menu.find(m => m.key === section)

    if (active) {
      return (
        <Stack gap="md">
          <Group gap="xs" wrap="nowrap">
            <ActionIcon variant="subtle" aria-label={t('common.back')} onClick={() => setSearchParams({})}>
              <IconChevronLeft size={18} />
            </ActionIcon>
            <Title order={3}>{active.label}</Title>
          </Group>
          {active.content}
        </Stack>
      )
    }

    return (
      <Stack gap="md">
        <Title order={2}>{t('settings.title')}</Title>
        <MobileList>
          {menu.map(m => (
            <MobileListRow
              key={m.key}
              leftSection={<m.icon size={18} />}
              title={m.label}
              chevron
              onClick={() => setSearchParams({ section: m.key })}
            />
          ))}
        </MobileList>
      </Stack>
    )
  }

  return (
    <>
      <Title order={2} mb="md">{t('settings.title')}</Title>
      <Tabs defaultValue="categories">
        <Tabs.List>
          <Tabs.Tab value="categories">{t('settings.categories')}</Tabs.Tab>
          <Tabs.Tab value="sources">{t('settings.incomeSources')}</Tabs.Tab>
          <Tabs.Tab value="pots">{t('settings.pots')}</Tabs.Tab>
          <Tabs.Tab value="years">{t('settings.years')}</Tabs.Tab>
          <Tabs.Tab value="exportImport">{t('settings.exportImport')}</Tabs.Tab>
          <Tabs.Tab value="preferences">{t('settings.preferences')}</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="categories" pt="md"><CategoriesTab /></Tabs.Panel>
        <Tabs.Panel value="sources" pt="md"><SourcesTab /></Tabs.Panel>
        <Tabs.Panel value="pots" pt="md"><PotsTab /></Tabs.Panel>
        <Tabs.Panel value="years" pt="md"><YearsTab /></Tabs.Panel>
        <Tabs.Panel value="exportImport" pt="md"><ExportImportTab /></Tabs.Panel>
        <Tabs.Panel value="preferences" pt="md"><PreferencesTab /></Tabs.Panel>
      </Tabs>
    </>
  )
}

function PreferencesTab() {
  const { t, i18n } = useTranslation()
  const { colorScheme, setColorScheme } = useMantineColorScheme()

  return (
    <Stack gap="lg" maw={360}>
      <Stack gap="xs">
        <Text size="sm" fw={600}>{t('common.theme')}</Text>
        <SegmentedControl
          value={colorScheme}
          onChange={(v) => setColorScheme(v as 'light' | 'dark' | 'auto')}
          aria-label={t('common.theme')}
          fullWidth
          data={[
            { label: <Group gap={6} justify="center"><IconSun size={16} />{t('common.themeLight')}</Group>, value: 'light' },
            { label: <Group gap={6} justify="center"><IconMoon size={16} />{t('common.themeDark')}</Group>, value: 'dark' },
            { label: <Group gap={6} justify="center"><IconDeviceDesktop size={16} />{t('common.themeAuto')}</Group>, value: 'auto' },
          ]}
        />
      </Stack>
      <Stack gap="xs">
        <Text size="sm" fw={600}>{t('common.language')}</Text>
        <Select
          aria-label={t('common.language')}
          data={[{ value: 'nl', label: 'Nederlands' }, { value: 'en', label: 'English' }]}
          value={i18n.language.startsWith('nl') ? 'nl' : 'en'}
          onChange={(v) => v && i18n.changeLanguage(v)}
        />
      </Stack>
    </Stack>
  )
}

type CategorySortKey = 'name' | 'default' | 'itemized' | 'template'

function CategoriesTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
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

  if (isMobile) {
    const editingCat = rows.find(c => c.id === editing)
    return (
      <Stack gap="sm">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <Group justify="space-between">
          <SortMenu
            sort={sort}
            onSort={k => setSort(s => toggleSort(s, k))}
            options={[
              { key: 'name', label: t('settings.name') },
              { key: 'default', label: t('settings.default') },
              { key: 'itemized', label: t('settings.itemized') },
              { key: 'template', label: t('settings.template') },
            ]}
          />
          <ActionIcon variant="filled" aria-label={t('common.add')} onClick={openAdd}><IconPlus size={16} /></ActionIcon>
        </Group>
        {rows.length === 0 ? (
          <EmptyState message={t('settings.noCategories')} />
        ) : (
          <MobileList>
            {rows.map(cat => (
              <MobileListRow
                key={cat.id}
                title={<>{cat.name} {cat.archivedAt && <Badge size="xs" color="gray" ml="xs">{t('settings.archived')}</Badge>}</>}
                subtitle={[cat.isItemized && t('settings.itemized'), cat.includeInTemplate && t('settings.template')].filter(Boolean).join(' · ')}
                trailing={<Text size="sm">€ {(cat.defaultAmountCents / 100).toFixed(2)}</Text>}
                chevron
                onClick={() => startEdit(cat)}
              />
            ))}
          </MobileList>
        )}

        <BottomSheet opened={editing !== null} onClose={() => setEditing(null)} title={t('common.edit')}>
          {editingCat && (
            <>
              <TextInput label={t('settings.name')} value={editName} onChange={e => setEditName(e.target.value)} />
              <NumberInput label={t('settings.default')} value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
              <Switch label={t('settings.itemized')} checked={editItemized} onChange={e => setEditItemized(e.target.checked)} />
              <Switch label={t('settings.template')} checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} />
              <Group grow>
                <Button onClick={saveEdit} loading={update.isPending}>{t('common.save')}</Button>
                <Button
                  color={editingCat.archivedAt ? 'green' : 'red'}
                  variant="light"
                  loading={archive.isPending}
                  onClick={() => archive.mutate(editingCat.id, { onSuccess: () => setEditing(null) })}
                >
                  {editingCat.archivedAt ? t('settings.restore') : t('common.archive')}
                </Button>
              </Group>
            </>
          )}
        </BottomSheet>

        <BottomSheet opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
          <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
          <NumberInput label={t('settings.default')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
          <Switch label={t('settings.itemized')} checked={newItemized} onChange={e => setNewItemized(e.target.checked)} />
          <Switch label={t('settings.template')} checked={newTemplate} onChange={e => setNewTemplate(e.target.checked)} />
          <Button disabled={!newName} loading={create.isPending} onClick={() => {
            create.mutate({ name: newName, defaultAmountCents: amountToCents(newAmount), isItemized: newItemized, includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0) }, {
              onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false); setNewTemplate(true); closeAdd() }
            })
          }}>{t('common.add')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

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
      <Table.ScrollContainer minWidth={520}>
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
                    <Table.Td><Group justify="flex-end"><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editItemized} onChange={e => setEditItemized(e.target.checked)} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} /></Group></Table.Td>
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
      </Table.ScrollContainer>
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
  const isMobile = useMediaQuery('(max-width: 47.99em)')
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

  const startEdit = (src: { id: number; name: string; defaultAmountCents: number; includeInTemplate: boolean }) => {
    setEditing(src.id)
    setEditName(src.name)
    setEditAmount(src.defaultAmountCents / 100)
    setEditTemplate(src.includeInTemplate)
  }

  if (isMobile) {
    const editingSrc = rows.find(s => s.id === editing)
    return (
      <Stack gap="sm">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <Group justify="space-between">
          <SortMenu
            sort={sort}
            onSort={k => setSort(s => toggleSort(s, k))}
            options={[
              { key: 'name', label: t('settings.name') },
              { key: 'default', label: t('settings.default') },
              { key: 'template', label: t('settings.template') },
            ]}
          />
          <ActionIcon variant="filled" aria-label={t('common.add')} onClick={openAdd}><IconPlus size={16} /></ActionIcon>
        </Group>
        {rows.length === 0 ? (
          <EmptyState message={t('settings.noSources')} />
        ) : (
          <MobileList>
            {rows.map(src => (
              <MobileListRow
                key={src.id}
                title={<>{src.name} {src.archivedAt && <Badge size="xs" color="gray" ml="xs">{t('settings.archived')}</Badge>}</>}
                subtitle={src.includeInTemplate ? t('settings.template') : undefined}
                trailing={<Text size="sm">€ {(src.defaultAmountCents / 100).toFixed(2)}</Text>}
                chevron
                onClick={() => startEdit(src)}
              />
            ))}
          </MobileList>
        )}

        <BottomSheet opened={editing !== null} onClose={() => setEditing(null)} title={t('common.edit')}>
          {editingSrc && (
            <>
              <TextInput label={t('settings.name')} value={editName} onChange={e => setEditName(e.target.value)} />
              <NumberInput label={t('settings.default')} value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
              <Switch label={t('settings.template')} checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} />
              <Group grow>
                <Button
                  onClick={() => update.mutate({ id: editingSrc.id, name: editName, defaultAmountCents: amountToCents(editAmount), includeInTemplate: editTemplate, sortOrder: editingSrc.sortOrder }, { onSuccess: () => setEditing(null) })}
                  loading={update.isPending}
                >
                  {t('common.save')}
                </Button>
                <Button
                  color={editingSrc.archivedAt ? 'green' : 'red'}
                  variant="light"
                  loading={archive.isPending}
                  onClick={() => archive.mutate(editingSrc.id, { onSuccess: () => setEditing(null) })}
                >
                  {editingSrc.archivedAt ? t('settings.restore') : t('common.archive')}
                </Button>
              </Group>
            </>
          )}
        </BottomSheet>

        <BottomSheet opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
          <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
          <NumberInput label={t('settings.default')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
          <Switch label={t('settings.template')} checked={newTemplate} onChange={e => setNewTemplate(e.target.checked)} />
          <Button disabled={!newName} loading={create.isPending} onClick={() => {
            create.mutate({ name: newName, defaultAmountCents: amountToCents(newAmount), includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0) }, {
              onSuccess: () => { setNewName(''); setNewAmount(''); setNewTemplate(true); closeAdd() }
            })
          }}>{t('common.add')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

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
      <Table.ScrollContainer minWidth={480}>
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
                    <Table.Td><Group justify="flex-end"><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} /></Group></Table.Td>
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
      </Table.ScrollContainer>
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
  const isMobile = useMediaQuery('(max-width: 47.99em)')
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

  const startEdit = (pot: { id: number; name: string; kind: string; targetCents?: number | null; targetDate?: string | null }) => {
    setEditing(pot.id)
    setEditName(pot.name)
    setEditKind(pot.kind)
    setEditTarget(pot.targetCents != null ? pot.targetCents / 100 : '')
    setEditTargetDate(pot.targetDate ?? null)
  }

  const saveEdit = (pot: { id: number; sortOrder: number }) => {
    update.mutate({
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
    })
  }

  if (isMobile) {
    const editingPot = rows.find(p => p.id === editing)
    return (
      <Stack gap="sm">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <Group justify="space-between">
          <SortMenu
            sort={sort}
            onSort={k => setSort(s => toggleSort(s, k))}
            options={[
              { key: 'name', label: t('settings.name') },
              { key: 'kind', label: t('settings.type') },
              { key: 'target', label: t('pots.target') },
            ]}
          />
          <ActionIcon variant="filled" aria-label={t('common.add')} onClick={openAdd}><IconPlus size={16} /></ActionIcon>
        </Group>
        {rows.length === 0 ? (
          <EmptyState message={t('settings.noPots')} />
        ) : (
          <MobileList>
            {rows.map(pot => (
              <MobileListRow
                key={pot.id}
                title={<>{pot.name} {pot.archivedAt && <Badge size="xs" color="gray" ml="xs">{t('settings.archived')}</Badge>}</>}
                subtitle={kindLabel(pot.kind)}
                trailing={pot.targetCents != null ? <MoneyText cents={pot.targetCents} size="sm" /> : undefined}
                chevron
                onClick={() => startEdit(pot)}
              />
            ))}
          </MobileList>
        )}

        <BottomSheet opened={editing !== null} onClose={() => setEditing(null)} title={t('common.edit')}>
          {editingPot && (
            <>
              <TextInput label={t('settings.name')} value={editName} onChange={e => setEditName(e.target.value)} />
              <Select
                label={t('settings.type')}
                value={editKind}
                onChange={v => setEditKind(v ?? 'normal')}
                data={[
                  { value: 'normal', label: t('settings.kind_normal') },
                  { value: 'carryover', label: t('settings.kind_carryover') },
                ]}
              />
              {editKind === 'normal' && (
                <>
                  <NumberInput
                    label={t('pots.target')}
                    value={editTarget}
                    onChange={setEditTarget}
                    decimalSeparator=","
                    decimalScale={2}
                    prefix="€ "
                    hideControls
                    placeholder="0,00"
                  />
                  <DateInput
                    label={t('pots.targetDate')}
                    value={editTargetDate}
                    onChange={setEditTargetDate}
                    valueFormat="DD-MM-YYYY"
                    clearable
                  />
                </>
              )}
              <Group grow>
                <Button onClick={() => saveEdit(editingPot)} loading={update.isPending}>{t('common.save')}</Button>
                <Button
                  color={editingPot.archivedAt ? 'green' : 'red'}
                  variant="light"
                  loading={archive.isPending}
                  onClick={() => archive.mutate(editingPot.id, { onSuccess: () => setEditing(null) })}
                >
                  {editingPot.archivedAt ? t('settings.restore') : t('common.archive')}
                </Button>
              </Group>
            </>
          )}
        </BottomSheet>

        <BottomSheet opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
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
          <Button disabled={!newName} loading={create.isPending} onClick={handleCreate}>{t('common.add')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

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
      <Table.ScrollContainer minWidth={480}>
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
      </Table.ScrollContainer>
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
  const isMobile = useMediaQuery('(max-width: 47.99em)')
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

  if (isMobile) {
    return (
      <Stack gap="sm">
        <TextInput
          placeholder={t('common.search')}
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <Group justify="space-between">
          <Button size="xs" variant="subtle" rightSection={sortDir === 'asc' ? <IconChevronUp size={13} /> : <IconChevronDown size={13} />} onClick={() => setSortDir(d => d === 'asc' ? 'desc' : 'asc')}>
            {t('settings.year')}
          </Button>
          <ActionIcon variant="filled" aria-label={t('common.add')} onClick={openAdd}><IconPlus size={16} /></ActionIcon>
        </Group>
        {rows.length === 0 ? (
          <EmptyState message={t('settings.noYears')} />
        ) : (
          <MobileList>
            {rows.map(y => <MobileListRow key={y} title={String(y)} />)}
          </MobileList>
        )}
        <BottomSheet opened={addOpened} onClose={closeAdd} title={t('common.addNew')}>
          <NumberInput label={t('settings.year')} value={newYear} onChange={setNewYear} hideControls decimalScale={0} />
          <Button disabled={!newYear} loading={create.isPending} onClick={() => {
            create.mutate(Number(newYear), { onSuccess: () => { setNewYear(''); closeAdd() } })
          }}>{t('common.add')}</Button>
        </BottomSheet>
      </Stack>
    )
  }

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

const EXPORT_MONTH_NL = ['', 'Januari', 'Februari', 'Maart', 'April', 'Mei', 'Juni', 'Juli', 'Augustus', 'September', 'Oktober', 'November', 'December']
const EXPORT_MONTH_EN = ['', 'January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

function ExportImportTab() {
  const { t, i18n } = useTranslation()
  const { data: years } = useYears()
  const currentYear = useCurrentYear()
  const [year, setYear] = useState<number>(currentYear)
  const [scope, setScope] = useState<'year' | 'months'>('year')
  const [months, setMonths] = useState<string[]>([])

  const monthNames = i18n.language.startsWith('nl') ? EXPORT_MONTH_NL : EXPORT_MONTH_EN
  const monthOptions = monthNames.slice(1).map((label, i) => ({ value: String(i + 1), label }))

  const handleDownload = () => {
    const query = scope === 'months' && months.length > 0 ? `?months=${months.join(',')}` : ''
    const a = document.createElement('a')
    a.href = `/api/export/years/${year}${query}`
    document.body.appendChild(a)
    a.click()
    a.remove()
    notifications.show({ color: 'green', message: t('export.downloadStarted') })
  }

  return (
    <Stack gap="md" maw={420}>
      <Text size="sm" c="dimmed">{t('export.description')}</Text>
      <Select
        label={t('settings.year')}
        data={(years ?? []).map(String)}
        value={String(year)}
        onChange={v => v && setYear(Number(v))}
      />
      <Stack gap="xs">
        <Text size="sm" fw={600}>{t('export.scope')}</Text>
        <SegmentedControl
          value={scope}
          onChange={v => setScope(v as 'year' | 'months')}
          fullWidth
          data={[
            { label: t('export.scopeWholeYear'), value: 'year' },
            { label: t('export.scopeMonths'), value: 'months' },
          ]}
        />
      </Stack>
      {scope === 'months' && (
        <MultiSelect
          label={t('export.months')}
          placeholder={t('export.scopeMonths')}
          data={monthOptions}
          value={months}
          onChange={setMonths}
        />
      )}
      <Button
        leftSection={<IconDownload size={16} />}
        disabled={scope === 'months' && months.length === 0}
        onClick={handleDownload}
      >
        {t('export.download')}
      </Button>

      <Divider my="sm" />

      <ImportSection />
    </Stack>
  )
}

function ImportSection() {
  const { t } = useTranslation()
  const currentYear = useCurrentYear()
  const importMutation = useImportXLSX()
  const [file, setFile] = useState<File | null>(null)
  const [importYear, setImportYear] = useState<number | string>(currentYear)
  const [wipe, setWipe] = useState(false)
  const [resetMaster, setResetMaster] = useState(false)
  const [closeThrough, setCloseThrough] = useState<number | string>(0)
  const [pwOpened, { open: openPw, close: closePw }] = useDisclosure(false)
  const [report, setReport] = useState<ImportReport | null>(null)

  const runImport = (password?: string) => {
    if (!file || !importYear) return
    importMutation.mutate({
      file,
      year: Number(importYear),
      wipe,
      resetMaster,
      closeThrough: Number(closeThrough) || 0,
      password,
    }, {
      onSuccess: (data) => {
        setReport(data)
        closePw()
        notifications.show({ color: 'green', message: t('export.importSuccess') })
      },
      onError: (err: unknown) => {
        notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) })
      },
    })
  }

  const handleImportClick = () => {
    if (wipe || resetMaster) {
      openPw()
      return
    }
    runImport()
  }

  return (
    <Stack gap="md">
      <Text size="sm" fw={600}>{t('export.importTitle')}</Text>
      <Text size="sm" c="dimmed">{t('export.importDescription')}</Text>
      <FileInput
        label={t('export.file')}
        placeholder={t('export.filePlaceholder')}
        leftSection={<IconUpload size={16} />}
        accept=".xlsx"
        value={file}
        onChange={setFile}
        clearable
      />
      <NumberInput
        label={t('settings.year')}
        value={importYear}
        onChange={setImportYear}
        hideControls
        decimalScale={0}
      />
      <NumberInput
        label={t('export.closeThrough')}
        description={t('export.closeThroughHint')}
        value={closeThrough}
        onChange={setCloseThrough}
        min={0}
        max={12}
        hideControls
      />
      <Switch
        label={t('export.wipe')}
        description={t('export.wipeHint')}
        checked={wipe}
        onChange={e => setWipe(e.target.checked)}
      />
      <Switch
        color="red"
        label={t('export.resetMaster')}
        description={t('export.resetMasterHint')}
        checked={resetMaster}
        onChange={e => setResetMaster(e.target.checked)}
      />
      <Button
        leftSection={<IconUpload size={16} />}
        color={wipe || resetMaster ? 'red' : undefined}
        disabled={!file || !importYear}
        loading={importMutation.isPending}
        onClick={handleImportClick}
      >
        {t('export.importButton')}
      </Button>

      {report && (
        <Table mt="sm">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('export.month')}</Table.Th>
              <Table.Th ta="right">{t('export.income')}</Table.Th>
              <Table.Th ta="right">{t('export.expense')}</Table.Th>
              <Table.Th ta="right">{t('export.surplus')}</Table.Th>
              <Table.Th>{t('common.closed')}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {report.months.map(m => (
              <Table.Tr key={m.month}>
                <Table.Td>{m.month}</Table.Td>
                <Table.Td ta="right"><MoneyText cents={m.incomeTotalCents} size="sm" /></Table.Td>
                <Table.Td ta="right"><MoneyText cents={m.expenseTotalCents} size="sm" /></Table.Td>
                <Table.Td ta="right"><MoneyText cents={m.surplusCents} size="sm" /></Table.Td>
                <Table.Td>{m.closed ? '✓' : ''}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      <PasswordModal
        opened={pwOpened}
        onClose={closePw}
        title={t('export.confirmDestructive')}
        warningText={resetMaster ? t('export.resetMasterWarning') : t('export.wipeWarning')}
        confirmLabel={t('export.importButton')}
        loading={importMutation.isPending}
        onConfirm={(password) => runImport(password)}
      />
    </Stack>
  )
}
