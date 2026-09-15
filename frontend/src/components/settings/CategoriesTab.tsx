import { useMemo, useState } from 'react'
import { ActionIcon, Badge, Button, Divider, Group, NumberInput, Select, Stack, Switch, Table, Text, TextInput, Tooltip } from '@mantine/core'
import { modals } from '@mantine/modals'
import { useMediaQuery } from '@mantine/hooks'
import { IconPlus, IconSearch, IconTrash, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import {
  useCategories, useCreateCategory, useUpdateCategory, useArchiveCategory,
  useCategoryAliases, useCreateCategoryAlias, useDeleteCategoryAlias,
} from '../../api/hooks/useSettings'
import MoneyText from '../MoneyText'
import EmptyState from '../EmptyState'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import BottomSheet from '../mobile/BottomSheet'
import { CategoryPresetsButton } from './DescriptionPresets'
import { SortableTh, SortMenu, cmp } from './SortControls'
import { amountToCents, ListSkeleton, LoadErrorAlert } from './common'
import { useEditableListState } from './useEditableListState'
import EntityAddForm from './EntityAddForm'

type CategorySortKey = 'name' | 'default' | 'itemized' | 'template'

interface CategoryRow { id: number; parentId?: number }

// Children sort immediately after their parent so the hierarchy reads
// top-to-bottom; a child whose parent got filtered out by search still shows
// up, just without the grouping.
function orderByHierarchy<T extends CategoryRow>(sorted: T[]): T[] {
  const childrenByParent = new Map<number, T[]>()
  const topLevel: T[] = []
  for (const c of sorted) {
    if (c.parentId) {
      const arr = childrenByParent.get(c.parentId) ?? []
      arr.push(c)
      childrenByParent.set(c.parentId, arr)
    } else {
      topLevel.push(c)
    }
  }
  const ordered: T[] = []
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
}

export default function CategoriesTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, isError, refetch } = useCategories()
  const create = useCreateCategory()
  const update = useUpdateCategory()
  const archive = useArchiveCategory()
  const [editName, setEditName] = useState('')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [editItemized, setEditItemized] = useState(false)
  const [editTemplate, setEditTemplate] = useState(true)
  const [editParentId, setEditParentId] = useState<string | null>(null)
  const [editAutofill, setEditAutofill] = useState(false)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newItemized, setNewItemized] = useState(false)
  const [newTemplate, setNewTemplate] = useState(true)
  const [newParentId, setNewParentId] = useState<string | null>(null)
  const [newAutofill, setNewAutofill] = useState(false)

  const { search, setSearch, sort, onSort, editing, setEditing, addOpened, openAdd, closeAdd, rows } =
    useEditableListState({
      data,
      initialSort: { key: 'name' as CategorySortKey, dir: 'asc' },
      matches: (c, q) => c.name.toLowerCase().includes(q.toLowerCase()),
      compare: (a, b, key) =>
        key === 'name' ? cmp(a.name, b.name)
          : key === 'default' ? cmp(a.defaultAmountCents, b.defaultAmountCents)
          : key === 'itemized' ? cmp(Number(a.isItemized), Number(b.isItemized))
          : cmp(Number(a.includeInTemplate), Number(b.includeInTemplate)),
      order: orderByHierarchy,
    })

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

  const startEdit = (cat: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; parentId?: number; autofillActual?: boolean }) => {
    setEditing(cat.id)
    setEditName(cat.name)
    setEditAmount(cat.defaultAmountCents / 100)
    setEditItemized(cat.isItemized)
    setEditTemplate(cat.includeInTemplate)
    setEditParentId(cat.parentId ? String(cat.parentId) : null)
    setEditAutofill(cat.autofillActual ?? false)
  }

  const saveEdit = () => {
    if (!editing || !editName.trim()) return
    update.mutate({
      id: editing, name: editName.trim(), defaultAmountCents: amountToCents(editAmount), isItemized: editItemized,
      includeInTemplate: editTemplate, parentId: editParentId ? Number(editParentId) : null, autofillActual: editAutofill,
    }, {
      onSuccess: () => setEditing(null)
    })
  }

  if (isLoading) return <ListSkeleton />
  if (isError) return <LoadErrorAlert onRetry={() => refetch()} />

  const addForm = (
    <EntityAddForm
      opened={addOpened}
      onClose={closeAdd}
      isMobile={isMobile}
      disabled={!newName.trim()}
      loading={create.isPending}
      onSubmit={() => create.mutate({
        name: newName.trim(), defaultAmountCents: amountToCents(newAmount), isItemized: newItemized,
        includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0), parentId: newParentId ? Number(newParentId) : null, autofillActual: newAutofill,
      }, {
        onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false); setNewTemplate(true); setNewParentId(null); setNewAutofill(false); closeAdd() },
      })}
    >
      <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
      <NumberInput label={t('settings.default')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
      <Switch label={t('settings.itemized')} checked={newItemized} onChange={e => setNewItemized(e.target.checked)} />
      <Switch label={t('settings.template')} checked={newTemplate} onChange={e => setNewTemplate(e.target.checked)} />
      <Switch label={t('settings.autofillActual')} checked={newAutofill} onChange={e => setNewAutofill(e.target.checked)} />
      <Select
        label={t('settings.parentCategory')}
        placeholder={t('settings.parentCategoryNone')}
        clearable
        data={parentOptions}
        value={newParentId}
        onChange={setNewParentId}
      />
    </EntityAddForm>
  )

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
            onSort={onSort}
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
                trailing={<MoneyText cents={cat.defaultAmountCents} size="sm" />}
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
              <Switch label={t('settings.autofillActual')} checked={editAutofill} onChange={e => setEditAutofill(e.target.checked)} />
              <Group justify="space-between">
                <Text size="sm">{t('settings.suggestions')}</Text>
                <CategoryPresetsButton categoryId={editingCat.id} />
              </Group>
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
                <Button onClick={saveEdit} loading={update.isPending} disabled={!editName.trim()}>{t('common.save')}</Button>
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

        {addForm}
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
            <SortableTh label={t('settings.name')} sortKey="name" sort={sort} onSort={onSort} />
            <SortableTh label={t('settings.default')} sortKey="default" sort={sort} onSort={onSort} ta="right" />
            <SortableTh label={t('settings.itemized')} sortKey="itemized" sort={sort} onSort={onSort} ta="center" />
            <SortableTh
              label={<Tooltip label={t('settings.templateHint')}><span>{t('settings.template')}</span></Tooltip>}
              sortKey="template" sort={sort} onSort={onSort} ta="center"
            />
            <Table.Th ta="center"><Tooltip label={t('settings.autofillActualHint')}><span>{t('settings.autofillActual')}</span></Tooltip></Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {rows.length === 0 && (
            <Table.Tr><Table.Td colSpan={6}><EmptyState message={t('settings.noCategories')} /></Table.Td></Table.Tr>
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
                      <Group justify="center">
                        <Switch checked={editAutofill} onChange={e => setEditAutofill(e.target.checked)} />
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" disabled={!editName.trim()} onClick={saveEdit}>{t('common.ok')}</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}><IconX size={14} /></Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{cat.parentId && '↳ '}{cat.name} {cat.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
                    <Table.Td ta="right"><MoneyText cents={cat.defaultAmountCents} size="sm" /></Table.Td>
                    <Table.Td ta="center">{cat.isItemized ? '✓' : ''}</Table.Td>
                    <Table.Td ta="center">{cat.includeInTemplate ? '✓' : '—'}</Table.Td>
                    <Table.Td ta="center">{cat.autofillActual ? '✓' : ''}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <CategoryPresetsButton categoryId={cat.id} />
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
      {addForm}

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

  const confirmDelete = (id: number) => modals.openConfirmModal({
    title: t('common.delete'),
    children: <Text size="sm">{t('common.confirm')}</Text>,
    labels: { confirm: t('common.delete'), cancel: t('common.cancel') },
    confirmProps: { color: 'red' },
    onConfirm: () => deleteAlias.mutate(id),
  })

  return (
    <Stack gap="sm" mt="lg">
      <Divider label={t('settings.categoryAliases')} />
      <Text size="sm" c="dimmed">{t('settings.categoryAliasesHint')}</Text>

      {aliases?.length === 0 && <EmptyState message={t('settings.noAliases')} />}
      {aliases?.map(a => (
        <Group key={a.id} justify="space-between" wrap="nowrap">
          <Text size="sm">{a.aliasName} → {categoryName(a.parentCategoryId)}</Text>
          <ActionIcon variant="subtle" color="red" aria-label={t('common.delete')} onClick={() => confirmDelete(a.id)}>
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
