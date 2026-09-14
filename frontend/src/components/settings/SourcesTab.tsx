import { useState } from 'react'
import { ActionIcon, Badge, Button, Group, NumberInput, Stack, Switch, Table, Text, TextInput, Tooltip } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { IconPlus, IconSearch, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import {
  useIncomeSources, useCreateIncomeSource, useUpdateIncomeSource, useArchiveIncomeSource,
} from '../../api/hooks/useSettings'
import MoneyText from '../MoneyText'
import EmptyState from '../EmptyState'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import BottomSheet from '../mobile/BottomSheet'
import { IncomeSourcePresetsButton } from './DescriptionPresets'
import { SortableTh, SortMenu, cmp } from './SortControls'
import { amountToCents, ListSkeleton, LoadErrorAlert } from './common'
import { useEditableListState } from './useEditableListState'
import EntityAddForm from './EntityAddForm'

type SourceSortKey = 'name' | 'default' | 'itemized' | 'template'

export default function SourcesTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, isError, refetch } = useIncomeSources()
  const create = useCreateIncomeSource()
  const update = useUpdateIncomeSource()
  const archive = useArchiveIncomeSource()
  const [editName, setEditName] = useState('')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [editItemized, setEditItemized] = useState(false)
  const [editTemplate, setEditTemplate] = useState(true)
  const [editAutofill, setEditAutofill] = useState(false)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newItemized, setNewItemized] = useState(false)
  const [newTemplate, setNewTemplate] = useState(true)
  const [newAutofill, setNewAutofill] = useState(false)

  const { search, setSearch, sort, onSort, editing, setEditing, addOpened, openAdd, closeAdd, rows } =
    useEditableListState({
      data,
      initialSort: { key: 'name' as SourceSortKey, dir: 'asc' },
      matches: (src, q) => src.name.toLowerCase().includes(q.toLowerCase()),
      compare: (a, b, key) =>
        key === 'name' ? cmp(a.name, b.name)
          : key === 'default' ? cmp(a.defaultAmountCents, b.defaultAmountCents)
          : key === 'itemized' ? cmp(Number(a.isItemized), Number(b.isItemized))
          : cmp(Number(a.includeInTemplate), Number(b.includeInTemplate)),
    })

  const startEdit = (src: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; autofillActual?: boolean }) => {
    setEditing(src.id)
    setEditName(src.name)
    setEditAmount(src.defaultAmountCents / 100)
    setEditItemized(src.isItemized)
    setEditTemplate(src.includeInTemplate)
    setEditAutofill(src.autofillActual ?? false)
  }

  const saveEdit = (src: { id: number; sortOrder: number }) => {
    if (!editName.trim()) return
    update.mutate({
      id: src.id, name: editName.trim(), defaultAmountCents: amountToCents(editAmount), isItemized: editItemized,
      includeInTemplate: editTemplate, sortOrder: src.sortOrder, autofillActual: editAutofill,
    }, { onSuccess: () => setEditing(null) })
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
        includeInTemplate: newTemplate, sortOrder: (data?.length ?? 0), autofillActual: newAutofill,
      }, {
        onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false); setNewTemplate(true); setNewAutofill(false); closeAdd() },
      })}
    >
      <TextInput label={t('settings.name')} value={newName} onChange={e => setNewName(e.target.value)} />
      <NumberInput label={t('settings.default')} value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls />
      <Switch label={t('settings.itemized')} checked={newItemized} onChange={e => setNewItemized(e.target.checked)} />
      <Switch label={t('settings.template')} checked={newTemplate} onChange={e => setNewTemplate(e.target.checked)} />
      <Switch label={t('settings.autofillActual')} checked={newAutofill} onChange={e => setNewAutofill(e.target.checked)} />
    </EntityAddForm>
  )

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
          <EmptyState message={t('settings.noSources')} />
        ) : (
          <MobileList>
            {rows.map(src => (
              <MobileListRow
                key={src.id}
                title={<>{src.name} {src.archivedAt && <Badge size="xs" color="gray" ml="xs">{t('settings.archived')}</Badge>}</>}
                subtitle={[src.isItemized && t('settings.itemized'), src.includeInTemplate && t('settings.template')].filter(Boolean).join(' · ')}
                trailing={<MoneyText cents={src.defaultAmountCents} size="sm" />}
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
              <Switch label={t('settings.autofillActual')} checked={editAutofill} onChange={e => setEditAutofill(e.target.checked)} />
              <Group justify="space-between">
                <Text size="sm">{t('settings.suggestions')}</Text>
                <IncomeSourcePresetsButton sourceId={editingSrc.id} />
              </Group>
              <Group grow>
                <Button
                  onClick={() => saveEdit(editingSrc)}
                  loading={update.isPending}
                  disabled={!editName.trim()}
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
      <Table.ScrollContainer minWidth={480}>
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
            <Table.Tr><Table.Td colSpan={6}><EmptyState message={t('settings.noSources')} /></Table.Td></Table.Tr>
          )}
          {rows.map(src => (
            <Table.Tr key={src.id} opacity={src.archivedAt ? 0.5 : 1}>
              {editing === src.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td><Group justify="flex-end"><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editItemized} onChange={e => setEditItemized(e.target.checked)} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editTemplate} onChange={e => setEditTemplate(e.target.checked)} /></Group></Table.Td>
                    <Table.Td><Group justify="center"><Switch checked={editAutofill} onChange={e => setEditAutofill(e.target.checked)} /></Group></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" disabled={!editName.trim()} onClick={() => saveEdit(src)}>{t('common.ok')}</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}><IconX size={14} /></Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{src.name} {src.archivedAt && <Badge size="xs" color="gray">{t('settings.archived')}</Badge>}</Table.Td>
                    <Table.Td ta="right"><MoneyText cents={src.defaultAmountCents} size="sm" /></Table.Td>
                    <Table.Td ta="center">{src.isItemized ? '✓' : ''}</Table.Td>
                    <Table.Td ta="center">{src.includeInTemplate ? '✓' : '—'}</Table.Td>
                    <Table.Td ta="center">{src.autofillActual ? '✓' : ''}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <IncomeSourcePresetsButton sourceId={src.id} />
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
      {addForm}
    </Stack>
  )
}
