import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Title, Tabs, Table, Button, Group, TextInput, NumberInput, Switch, Stack, Badge, Select, MultiSelect, Tooltip, Text, Modal, Menu, ActionIcon, SegmentedControl, Divider, FileInput, Avatar, Alert, useMantineColorScheme } from '@mantine/core'
import { modals } from '@mantine/modals'
import { useDebouncedValue, useDisclosure, useMediaQuery } from '@mantine/hooks'
import { DateInput } from '@mantine/dates'
import {
  IconX, IconSearch, IconPlus, IconChevronUp, IconChevronDown, IconArrowsSort,
  IconTags, IconCoin, IconPigMoney, IconCalendar, IconChevronLeft, IconPalette,
  IconSun, IconMoon, IconDeviceDesktop, IconFileSpreadsheet, IconDownload, IconUpload,
  IconUsers, IconUserCircle, IconApi, IconShieldCheck, IconHistory, IconAlertTriangle, IconTrash,
} from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import {
  useCategories, useCreateCategory, useUpdateCategory, useArchiveCategory,
  useCategoryAliases, useCreateCategoryAlias, useDeleteCategoryAlias,
  useIncomeSources, useCreateIncomeSource, useUpdateIncomeSource, useArchiveIncomeSource,
  usePots, useCreatePot, useUpdatePot, useArchivePot,
} from '../api/hooks/useSettings'
import { useYears, useCreateYear, useImportXLSX } from '../api/hooks/usePeriods'
import { useCurrentYear } from '../api/hooks/useCurrentYear'
import { useMe } from '../api/hooks/useMe'
import { useUpdateMe, useUploadAvatar, useUsers, useUpdateUserRole } from '../api/hooks/useUsers'
import { useAuditLog } from '../api/hooks/useAuditLog'
import { parseToCents } from '../lib/money'
import { getErrorMessage } from '../api/client'
import type { PaletteId } from '../lib/chartPalette'
import { CHART_PALETTES, PALETTE_IDS } from '../lib/chartPalette'
import { useChartPalette } from '../contexts/ChartPaletteContext'
import type { ImportReport } from '../api/types'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'
import ReauthConfirmModal from '../components/ReauthConfirmModal'
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

type SettingsSection = 'categories' | 'sources' | 'pots' | 'years' | 'exportImport' | 'users' | 'auditLog' | 'apiDocs' | 'profile' | 'preferences'

interface SettingsMenuItem {
  key: SettingsSection
  label: string
  icon: typeof IconTags
  content: React.ReactNode
  adminOnly?: boolean
  external?: string
}

export default function Settings() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const [searchParams, setSearchParams] = useSearchParams()
  const section = searchParams.get('section') as SettingsSection | null
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  // Family-finance admin sections, then sections everyone gets, then the
  // API docs developer resource last — visually separated (Divider/gap)
  // since it's not a family-finance admin task, just a technical reference.
  const allMenuItems: SettingsMenuItem[] = [
    { key: 'categories', label: t('settings.categories'), icon: IconTags, content: <CategoriesTab />, adminOnly: true },
    { key: 'sources', label: t('settings.incomeSources'), icon: IconCoin, content: <SourcesTab />, adminOnly: true },
    { key: 'pots', label: t('settings.pots'), icon: IconPigMoney, content: <PotsTab />, adminOnly: true },
    { key: 'years', label: t('settings.years'), icon: IconCalendar, content: <YearsTab />, adminOnly: true },
    { key: 'exportImport', label: t('settings.exportImport'), icon: IconFileSpreadsheet, content: <ExportImportTab />, adminOnly: true },
    { key: 'users', label: t('settings.users'), icon: IconUsers, content: <UsersTab />, adminOnly: true },
    { key: 'auditLog', label: t('settings.auditLog'), icon: IconHistory, content: <AuditLogTab />, adminOnly: true },
    { key: 'profile', label: t('settings.profile'), icon: IconUserCircle, content: <ProfileTab /> },
    { key: 'preferences', label: t('settings.preferences'), icon: IconPalette, content: <PreferencesTab /> },
    { key: 'apiDocs', label: t('nav.apiDocs'), icon: IconApi, content: null, adminOnly: true, external: '/api/docs' },
  ]
  const menu = allMenuItems.filter(m => isAdmin || !m.adminOnly)

  if (isMobile) {
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
          {menu.filter(m => !m.external).map(m => (
            <MobileListRow
              key={m.key}
              leftSection={<m.icon size={18} />}
              title={m.label}
              chevron
              onClick={() => setSearchParams({ section: m.key })}
            />
          ))}
        </MobileList>
        {menu.filter(m => m.external).map(m => (
          <Stack key={m.key} gap="md">
            <Divider />
            <MobileList>
              <MobileListRow
                leftSection={<m.icon size={18} />}
                title={m.label}
                onClick={() => window.open(m.external, '_blank', 'noopener,noreferrer')}
              />
            </MobileList>
          </Stack>
        ))}
      </Stack>
    )
  }

  const defaultTab = isAdmin ? 'categories' : 'profile'
  const apiDocsItem = menu.find(m => m.external)

  return (
    <>
      <Group justify="space-between" mb="md" wrap="wrap">
        <Title order={2}>{t('settings.title')}</Title>
        {apiDocsItem && (
          <Button
            variant="subtle"
            size="xs"
            color="gray"
            leftSection={<apiDocsItem.icon size={14} />}
            onClick={() => window.open(apiDocsItem.external, '_blank', 'noopener,noreferrer')}
          >
            {apiDocsItem.label}
          </Button>
        )}
      </Group>
      <Tabs defaultValue={defaultTab}>
        <Tabs.List>
          {menu.filter(m => !m.external).map(m => (
            <Tabs.Tab key={m.key} value={m.key}>{m.label}</Tabs.Tab>
          ))}
        </Tabs.List>
        {menu.filter(m => !m.external).map(m => (
          <Tabs.Panel key={m.key} value={m.key} pt="md">{m.content}</Tabs.Panel>
        ))}
      </Tabs>
    </>
  )
}

function PaletteDots({ paletteId }: { paletteId: PaletteId }) {
  const p = CHART_PALETTES[paletteId]
  return (
    <Group gap={4} justify="center" wrap="nowrap">
      {[p.income, p.expenses, p.surplus].map((color, i) => (
        <span key={i} style={{ width: 8, height: 8, borderRadius: '50%', flexShrink: 0, background: color.includes('.') ? `var(--mantine-color-${color.replace('.', '-')})` : color }} />
      ))}
    </Group>
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
  const [editParentId, setEditParentId] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newItemized, setNewItemized] = useState(false)
  const [newTemplate, setNewTemplate] = useState(true)
  const [newParentId, setNewParentId] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortState<CategorySortKey>>({ key: 'name', dir: 'asc' })
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  // Categories with a non-null parentId are themselves children — only
  // top-level categories are valid parent options (2-level max, enforced
  // again server-side).
  const parentOptions = useMemo(
    () => (data ?? []).filter(c => !c.parentId).map(c => ({ value: String(c.id), label: c.name })),
    [data]
  )
  const childCounts = useMemo(() => {
    const m = new Map<number, number>()
    for (const c of data ?? []) {
      if (c.parentId) m.set(c.parentId, (m.get(c.parentId) ?? 0) + 1)
    }
    return m
  }, [data])

  const startEdit = (cat: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; parentId?: number }) => {
    setEditing(cat.id)
    setEditName(cat.name)
    setEditAmount(cat.defaultAmountCents / 100)
    setEditItemized(cat.isItemized)
    setEditTemplate(cat.includeInTemplate)
    setEditParentId(cat.parentId ? String(cat.parentId) : null)
  }

  const saveEdit = () => {
    if (!editing) return
    update.mutate({
      id: editing, name: editName, defaultAmountCents: amountToCents(editAmount), isItemized: editItemized,
      includeInTemplate: editTemplate, parentId: editParentId ? Number(editParentId) : null,
    }, {
      onSuccess: () => setEditing(null)
    })
  }

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(c => c.name.toLowerCase().includes(search.toLowerCase()))
    const comparator = (a: typeof filtered[number], b: typeof filtered[number]) => {
      const c = sort.key === 'name' ? cmp(a.name, b.name)
        : sort.key === 'default' ? cmp(a.defaultAmountCents, b.defaultAmountCents)
        : sort.key === 'itemized' ? cmp(Number(a.isItemized), Number(b.isItemized))
        : cmp(Number(a.includeInTemplate), Number(b.includeInTemplate))
      return sort.dir === 'asc' ? c : -c
    }
    const sorted = [...filtered].sort(comparator)
    const childrenByParent = new Map<number, typeof sorted>()
    const topLevel: typeof sorted = []
    for (const c of sorted) {
      if (c.parentId) {
        const arr = childrenByParent.get(c.parentId) ?? []
        arr.push(c)
        childrenByParent.set(c.parentId, arr)
      } else {
        topLevel.push(c)
      }
    }
    // Children sort immediately after their parent so the hierarchy reads
    // top-to-bottom; a child whose parent got filtered out by search still
    // shows up, just without the grouping.
    const ordered: typeof sorted = []
    const seenParents = new Set<number>()
    for (const c of topLevel) {
      ordered.push(c)
      seenParents.add(c.id)
      const kids = childrenByParent.get(c.id)
      if (kids) ordered.push(...kids)
    }
    for (const [pid, kids] of childrenByParent) {
      if (!seenParents.has(pid)) ordered.push(...kids)
    }
    return ordered
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
                title={<>{cat.parentId && '↳ '}{cat.name} {cat.archivedAt && <Badge size="xs" color="gray" ml="xs">{t('settings.archived')}</Badge>}</>}
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
              {!childCounts.get(editingCat.id) && (
                <Select
                  label={t('settings.parentCategory')}
                  placeholder={t('settings.parentCategoryNone')}
                  clearable
                  data={parentOptions.filter(o => o.value !== String(editingCat.id))}
                  value={editParentId}
                  onChange={setEditParentId}
                />
              )}
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
          <Select
            label={t('settings.parentCategory')}
            placeholder={t('settings.parentCategoryNone')}
            clearable
            data={parentOptions}
            value={newParentId}
            onChange={setNewParentId}
          />
          <Button disabled={!newName} loading={create.isPending} onClick={() => {
            create.mutate({
              name: newName, defaultAmountCents: amountToCents(newAmount), isItemized: newItemized,
              includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0), parentId: newParentId ? Number(newParentId) : null,
            }, {
              onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false); setNewTemplate(true); setNewParentId(null); closeAdd() }
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
                    <Table.Td>
                      <Stack gap={4}>
                        <TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} />
                        {!childCounts.get(cat.id) && (
                          <Select
                            size="xs"
                            placeholder={t('settings.parentCategoryNone')}
                            clearable
                            data={parentOptions.filter(o => o.value !== String(cat.id))}
                            value={editParentId}
                            onChange={setEditParentId}
                          />
                        )}
                      </Stack>
                    </Table.Td>
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
                    <Table.Td>{cat.parentId && '↳ '}{cat.name} {cat.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
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
          <Select
            label={t('settings.parentCategory')}
            placeholder={t('settings.parentCategoryNone')}
            clearable
            data={parentOptions}
            value={newParentId}
            onChange={setNewParentId}
          />
          <Button disabled={!newName} loading={create.isPending} onClick={() => {
            create.mutate({
              name: newName, defaultAmountCents: amountToCents(newAmount), isItemized: newItemized,
              includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0), parentId: newParentId ? Number(newParentId) : null,
            }, {
              onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false); setNewTemplate(true); setNewParentId(null); closeAdd() }
            })
          }}>{t('common.add')}</Button>
        </Stack>
      </Modal>

      <CategoryAliasSection categories={data ?? []} />
    </Stack>
  )
}

function CategoryAliasSection({ categories }: { categories: { id: number; name: string; parentId?: number }[] }) {
  const { t } = useTranslation()
  const { data: aliases } = useCategoryAliases()
  const createAlias = useCreateCategoryAlias()
  const deleteAlias = useDeleteCategoryAlias()
  const [aliasName, setAliasName] = useState('')
  const [aliasParentId, setAliasParentId] = useState<string | null>(null)

  const parentOptions = useMemo(
    () => categories.filter(c => !c.parentId).map(c => ({ value: String(c.id), label: c.name })),
    [categories]
  )
  const categoryName = (id: number) => categories.find(c => c.id === id)?.name ?? `#${id}`

  return (
    <Stack gap="sm" mt="lg">
      <Divider label={t('settings.categoryAliases')} />
      <Text size="sm" c="dimmed">{t('settings.categoryAliasesHint')}</Text>

      {aliases?.length === 0 && <EmptyState message={t('settings.noAliases')} />}
      {aliases?.map(a => (
        <Group key={a.id} justify="space-between" wrap="nowrap">
          <Text size="sm">{a.aliasName} → {categoryName(a.parentCategoryId)}</Text>
          <ActionIcon variant="subtle" color="red" aria-label={t('common.delete')} onClick={() => deleteAlias.mutate(a.id)}>
            <IconTrash size={16} />
          </ActionIcon>
        </Group>
      ))}

      <Group align="flex-end" wrap="wrap">
        <TextInput
          label={t('settings.aliasName')}
          value={aliasName}
          onChange={e => setAliasName(e.target.value)}
          w={200}
        />
        <Select
          label={t('settings.parentCategory')}
          data={parentOptions}
          value={aliasParentId}
          onChange={setAliasParentId}
          w={200}
        />
        <Button
          disabled={!aliasName || !aliasParentId}
          loading={createAlias.isPending}
          onClick={() => {
            createAlias.mutate({ aliasName, parentCategoryId: Number(aliasParentId) }, {
              onSuccess: () => { setAliasName(''); setAliasParentId(null) }
            })
          }}
        >
          {t('common.add')}
        </Button>
      </Group>
    </Stack>
  )
}

type SourceSortKey = 'name' | 'default' | 'itemized' | 'template'

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
  const [editItemized, setEditItemized] = useState(false)
  const [editTemplate, setEditTemplate] = useState(true)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newItemized, setNewItemized] = useState(false)
  const [newTemplate, setNewTemplate] = useState(true)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortState<SourceSortKey>>({ key: 'name', dir: 'asc' })
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(s => s.name.toLowerCase().includes(search.toLowerCase()))
    return filtered.sort((a, b) => {
      const c = sort.key === 'name' ? cmp(a.name, b.name)
        : sort.key === 'default' ? cmp(a.defaultAmountCents, b.defaultAmountCents)
        : sort.key === 'itemized' ? cmp(Number(a.isItemized), Number(b.isItemized))
        : cmp(Number(a.includeInTemplate), Number(b.includeInTemplate))
      return sort.dir === 'asc' ? c : -c
    })
  }, [data, search, sort])

  const startEdit = (src: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean }) => {
    setEditing(src.id)
    setEditName(src.name)
    setEditAmount(src.defaultAmountCents / 100)
    setEditItemized(src.isItemized)
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
              { key: 'itemized', label: t('settings.itemized') },
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
                subtitle={[src.isItemized && t('settings.itemized'), src.includeInTemplate && t('settings.template')].filter(Boolean).join(' · ')}
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
              <Switch label={t('settings.itemized')} checked={editItemized} onChange={e => setEditItemized(e.target.checked)} />
              <Switch label={t('settings.template')} checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} />
              <Group grow>
                <Button
                  onClick={() => update.mutate({ id: editingSrc.id, name: editName, defaultAmountCents: amountToCents(editAmount), isItemized: editItemized, includeInTemplate: editTemplate, sortOrder: editingSrc.sortOrder }, { onSuccess: () => setEditing(null) })}
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
      <Table.ScrollContainer minWidth={480}>
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
            <Table.Tr><Table.Td colSpan={5}><EmptyState message={t('settings.noSources')} /></Table.Td></Table.Tr>
          )}
          {rows.map(src => (
            <Table.Tr key={src.id} opacity={src.archivedAt ? 0.5 : 1}>
              {editing === src.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td><Group justify="flex-end"><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editItemized} onChange={e => setEditItemized(e.target.checked)} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} /></Group></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={() => update.mutate({ id: src.id, name: editName, defaultAmountCents: amountToCents(editAmount), isItemized: editItemized, includeInTemplate: editTemplate, sortOrder: src.sortOrder }, { onSuccess: () => setEditing(null) })}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}><IconX size={14} /></Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{src.name} {src.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
                    <Table.Td ta="right">€ {(src.defaultAmountCents / 100).toFixed(2)}</Table.Td>
                    <Table.Td ta="center">{src.isItemized ? '✓' : ''}</Table.Td>
                    <Table.Td ta="center">{src.includeInTemplate ? '✓' : '—'}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => startEdit(src)}>{t('common.edit')}</Button>
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

  const runImport = () => {
    if (!file || !importYear) return
    importMutation.mutate({
      file,
      year: Number(importYear),
      wipe,
      resetMaster,
      closeThrough: Number(closeThrough) || 0,
    }, {
      onSuccess: (data) => {
        setReport(data)
        closePw()
        if (data.skippedSheets?.length) {
          notifications.show({ color: 'yellow', message: t('export.skippedSheets') + ' ' + data.skippedSheets.join(', ') })
        } else {
          notifications.show({ color: 'green', message: t('export.importSuccess') })
        }
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

      {report?.skippedSheets && report.skippedSheets.length > 0 && (
        <Alert color="yellow" icon={<IconAlertTriangle size={16} />} title={t('export.skippedSheets')}>
          {report.skippedSheets.join(', ')}
        </Alert>
      )}

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

      <ReauthConfirmModal
        opened={pwOpened}
        onClose={closePw}
        title={t('export.confirmDestructive')}
        warningText={resetMaster ? t('export.resetMasterWarning') : t('export.wipeWarning')}
        confirmLabel={t('export.importButton')}
        loading={importMutation.isPending}
        onConfirm={runImport}
      />
    </Stack>
  )
}

function ProfileTab() {
  const { t } = useTranslation()
  const { data: me } = useMe()
  const updateMe = useUpdateMe()
  const uploadAvatar = useUploadAvatar()
  const { paletteId, setPaletteId } = useChartPalette()
  const [name, setName] = useState('')

  useEffect(() => {
    if (me) setName(me.displayName)
  }, [me])

  const handleAvatarChange = (file: File | null) => {
    if (file) uploadAvatar.mutate(file)
  }

  return (
    <Stack gap="md" maw={360}>
      <Group>
        <Avatar src={me?.avatarUrl ?? undefined} size={64} radius="xl">
          {me?.displayName ? me.displayName[0].toUpperCase() : '?'}
        </Avatar>
        <FileInput
          placeholder={t('settings.uploadAvatar')}
          leftSection={<IconUpload size={16} />}
          accept="image/png,image/jpeg,image/webp"
          onChange={handleAvatarChange}
          loading={uploadAvatar.isPending}
          clearable
        />
      </Group>
      <TextInput label={t('settings.name')} value={name} onChange={e => setName(e.target.value)} />
      <Badge
        w="fit-content"
        color={me?.role === 'admin' ? 'teal' : 'gray'}
        leftSection={<IconShieldCheck size={12} />}
      >
        {me?.role === 'admin' ? t('settings.roleAdmin') : t('settings.roleUser')}
      </Badge>
      <Button onClick={() => updateMe.mutate(name)} loading={updateMe.isPending} disabled={!name}>
        {t('common.save')}
      </Button>

      <Divider my={4} />

      <Stack gap="xs">
        <Text size="sm" fw={600}>{t('settings.chartPalette')}</Text>
        <SegmentedControl
          value={paletteId}
          onChange={(v) => setPaletteId(v as PaletteId)}
          aria-label={t('settings.chartPalette')}
          fullWidth
          data={PALETTE_IDS.map((id) => ({
            value: id,
            label: (
              <Stack gap={2} align="center">
                <PaletteDots paletteId={id} />
                <Text size="xs">{t(`settings.palette_${id}`)}</Text>
              </Stack>
            ),
          }))}
        />
      </Stack>
    </Stack>
  )
}

function UsersTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data: me } = useMe()
  const { data: users } = useUsers()
  const updateRole = useUpdateUserRole()

  const roleLabel = (role: string) => role === 'admin' ? t('settings.roleAdmin') : t('settings.roleUser')

  const toggleRole = (u: { id: number; role: string; displayName: string; email: string }) => {
    const nextRole = u.role === 'admin' ? 'user' : 'admin'
    const applyChange = () => updateRole.mutate({ id: u.id, role: nextRole }, {
      onError: (err: unknown) => notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) }),
    })
    if (nextRole === 'user' && u.id === me?.id) {
      modals.openConfirmModal({
        title: t('settings.demoteSelfTitle'),
        children: <Text size="sm">{t('settings.demoteSelfConfirm')}</Text>,
        labels: { confirm: t('common.confirm'), cancel: t('common.cancel') },
        confirmProps: { color: 'red' },
        onConfirm: applyChange,
      })
      return
    }
    applyChange()
  }

  if (isMobile) {
    return (
      <MobileList>
        {(users ?? []).map(u => (
          <MobileListRow
            key={u.id}
            title={u.displayName || u.email}
            subtitle={u.email}
            trailing={<Badge color={u.role === 'admin' ? 'teal' : 'gray'} size="sm">{roleLabel(u.role)}</Badge>}
            onClick={() => toggleRole(u)}
          />
        ))}
      </MobileList>
    )
  }

  return (
    <Table.ScrollContainer minWidth={480}>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('settings.name')}</Table.Th>
            <Table.Th>{t('settings.email')}</Table.Th>
            <Table.Th>{t('settings.role')}</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(users ?? []).map(u => (
            <Table.Tr key={u.id}>
              <Table.Td>{u.displayName || '—'}</Table.Td>
              <Table.Td>{u.email}</Table.Td>
              <Table.Td><Badge color={u.role === 'admin' ? 'teal' : 'gray'}>{roleLabel(u.role)}</Badge></Table.Td>
              <Table.Td>
                <Button size="xs" variant="subtle" loading={updateRole.isPending} onClick={() => toggleRole(u)}>
                  {u.role === 'admin' ? t('settings.demote') : t('settings.promote')}
                </Button>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  )
}

function AuditLogTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const [action, setAction] = useState('')
  const [entityType, setEntityType] = useState('')
  const [userEmail, setUserEmail] = useState('')
  const [from, setFrom] = useState<string | null>(null)
  const [to, setTo] = useState<string | null>(null)

  // Debounce the free-text filters so typing "period.close" doesn't fire a
  // request per keystroke — exact-match filters can only hit on the final
  // string anyway.
  const [debouncedAction] = useDebouncedValue(action, 300)
  const [debouncedEntityType] = useDebouncedValue(entityType, 300)
  const [debouncedUserEmail] = useDebouncedValue(userEmail, 300)

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useAuditLog({
    action: debouncedAction, entityType: debouncedEntityType, userEmail: debouncedUserEmail, from, to,
  })
  const entries = data?.pages.flat() ?? []

  const entityLabel = (e: { entityType?: string; entityId?: number }) =>
    e.entityType ? `${e.entityType}${e.entityId != null ? ' #' + e.entityId : ''}` : '—'

  return (
    <Stack gap="sm">
      <Group wrap="wrap" gap="sm">
        <TextInput label={t('settings.auditLogAction')} value={action} onChange={e => setAction(e.target.value)} placeholder="period.close" w={180} />
        <TextInput label={t('settings.auditLogEntityType')} value={entityType} onChange={e => setEntityType(e.target.value)} placeholder="period" w={140} />
        <TextInput label={t('settings.auditLogUser')} value={userEmail} onChange={e => setUserEmail(e.target.value)} w={200} />
        <DateInput label={t('settings.auditLogFrom')} value={from} onChange={setFrom} valueFormat="DD-MM-YYYY" clearable w={140} />
        <DateInput label={t('settings.auditLogTo')} value={to} onChange={setTo} valueFormat="DD-MM-YYYY" clearable w={140} />
      </Group>

      {entries.length === 0 && !isLoading ? (
        <EmptyState message={t('settings.auditLogEmpty')} />
      ) : isMobile ? (
        <MobileList>
          {entries.map(e => (
            <MobileListRow
              key={e.id}
              title={e.action}
              subtitle={`${dayjs(e.createdAt).format('DD-MM-YYYY HH:mm')} · ${e.userEmail ?? '—'}`}
              trailing={<Badge size="xs" color="gray">{entityLabel(e)}</Badge>}
            />
          ))}
        </MobileList>
      ) : (
        <Table.ScrollContainer minWidth={700}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('settings.auditLogDate')}</Table.Th>
                <Table.Th>{t('settings.auditLogUser')}</Table.Th>
                <Table.Th>{t('settings.auditLogAction')}</Table.Th>
                <Table.Th>{t('settings.auditLogEntityType')}</Table.Th>
                <Table.Th>{t('settings.auditLogDetails')}</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {entries.map(e => (
                <Table.Tr key={e.id}>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{dayjs(e.createdAt).format('DD-MM-YYYY HH:mm')}</Table.Td>
                  <Table.Td>{e.userEmail ?? '—'}</Table.Td>
                  <Table.Td>{e.action}</Table.Td>
                  <Table.Td>{entityLabel(e)}</Table.Td>
                  <Table.Td><Text size="xs" c="dimmed" maw={300} style={{ whiteSpace: 'pre-wrap' }}>{e.details ?? ''}</Text></Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}

      {hasNextPage && (
        <Button variant="subtle" size="xs" onClick={() => fetchNextPage()} loading={isFetchingNextPage}>
          {t('settings.auditLogLoadMore')}
        </Button>
      )}
    </Stack>
  )
}
