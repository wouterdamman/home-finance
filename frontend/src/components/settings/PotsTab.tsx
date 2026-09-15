import { useState } from 'react'
import { ActionIcon, Badge, Button, Group, NumberInput, Select, Stack, Table, Text, TextInput } from '@mantine/core'
import { DateInput } from '@mantine/dates'
import { useMediaQuery } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import { IconPlus, IconSearch, IconX } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { usePots, useCreatePot, useUpdatePot, useArchivePot } from '../../api/hooks/useSettings'
import { getErrorMessage } from '../../api/client'
import MoneyText from '../MoneyText'
import EmptyState from '../EmptyState'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import BottomSheet from '../mobile/BottomSheet'
import { SortableTh, SortMenu, cmp } from './SortControls'
import { amountToCents, ListSkeleton, LoadErrorAlert } from './common'
import { useEditableListState } from './useEditableListState'
import EntityAddForm from './EntityAddForm'

type PotSortKey = 'name' | 'kind' | 'target'

export default function PotsTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, isError, refetch } = usePots()
  const create = useCreatePot()
  const update = useUpdatePot()
  const archive = useArchivePot()
  const [editName, setEditName] = useState('')
  const [editKind, setEditKind] = useState('normal')
  const [editTarget, setEditTarget] = useState<number | string>('')
  const [editTargetDate, setEditTargetDate] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newKind, setNewKind] = useState<string>('normal')
  const [newTarget, setNewTarget] = useState<number | string>('')
  const [newTargetDate, setNewTargetDate] = useState<string | null>(null)

  const { search, setSearch, sort, onSort, editing, setEditing, addOpened, openAdd, closeAdd, rows } =
    useEditableListState({
      data,
      initialSort: { key: 'name' as PotSortKey, dir: 'asc' },
      matches: (pot, q) => pot.name.toLowerCase().includes(q.toLowerCase()),
      compare: (a, b, key) =>
        key === 'name' ? cmp(a.name, b.name)
          : key === 'kind' ? cmp(a.kind, b.kind)
          : cmp(a.targetCents ?? -1, b.targetCents ?? -1),
    })

  const kindLabel = (kind: string) =>
    kind === 'carryover' ? t('settings.kind_carryover') : t('settings.kind_normal')

  // The "only one carryover pot" rule surfaces as the unique-index conflict
  // code; keying off the submitted kind alone reported it for every failure.
  const potErrorMessage = (err: unknown, kind: string) => {
    const code = (err as { body?: { error?: { code?: string } } })?.body?.error?.code
    return code === 'conflict' && kind === 'carryover'
      ? t('settings.carryoverExists')
      : getErrorMessage(err, t('common.error'))
  }

  const handleCreate = () => {
    if (!newName.trim()) return
    create.mutate({
      name: newName.trim(),
      kind: newKind,
      sortOrder: (data?.length ?? 0),
      targetCents: newKind === 'normal' && newTarget !== '' ? amountToCents(newTarget) : null,
      targetDate: newKind === 'normal' ? newTargetDate : null,
    }, {
      onSuccess: () => { setNewName(''); setNewKind('normal'); setNewTarget(''); setNewTargetDate(null); closeAdd() },
      onError: (err: unknown) => notifications.show({
        color: 'red',
        title: t('common.error'),
        message: potErrorMessage(err, newKind),
      }),
    })
  }

  const startEdit = (pot: { id: number; name: string; kind: string; targetCents?: number | null; targetDate?: string | null }) => {
    setEditing(pot.id)
    setEditName(pot.name)
    setEditKind(pot.kind)
    setEditTarget(pot.targetCents != null ? pot.targetCents / 100 : '')
    setEditTargetDate(pot.targetDate ?? null)
  }

  const saveEdit = (pot: { id: number; sortOrder: number }) => {
    if (!editName.trim()) return
    update.mutate({
      id: pot.id,
      name: editName.trim(),
      kind: editKind,
      sortOrder: pot.sortOrder,
      targetCents: editKind === 'normal' && editTarget !== '' ? amountToCents(editTarget) : null,
      targetDate: editKind === 'normal' ? editTargetDate : null,
    }, {
      onSuccess: () => setEditing(null),
      onError: (err: unknown) => notifications.show({
        color: 'red',
        title: t('common.error'),
        message: potErrorMessage(err, editKind),
      }),
    })
  }

  const addForm = (
    <EntityAddForm
      opened={addOpened}
      onClose={closeAdd}
      isMobile={isMobile}
      disabled={!newName.trim()}
      loading={create.isPending}
      onSubmit={handleCreate}
    >
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
    </EntityAddForm>
  )

  if (isLoading) return <ListSkeleton />
  if (isError) return <LoadErrorAlert onRetry={() => refetch()} />

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
            onSort={onSort}
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
                <Button onClick={() => saveEdit(editingPot)} loading={update.isPending} disabled={!editName.trim()}>{t('common.save')}</Button>
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
            <SortableTh label={t('settings.type')} sortKey="kind" sort={sort} onSort={onSort} />
            <SortableTh label={t('pots.target')} sortKey="target" sort={sort} onSort={onSort} />
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
                        <Button size="xs" disabled={!editName.trim()} onClick={() => saveEdit(pot)}>{t('common.ok')}</Button>
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
                        <Button size="xs" variant="subtle" onClick={() => startEdit(pot)}>{t('common.edit')}</Button>
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
      {addForm}
    </Stack>
  )
}
