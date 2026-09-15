import { useMemo, useState } from 'react'
import { ActionIcon, Button, Group, NumberInput, Stack, Table, TextInput } from '@mantine/core'
import { useDisclosure, useMediaQuery } from '@mantine/hooks'
import { IconChevronDown, IconChevronUp, IconPlus, IconSearch } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useYears, useCreateYear } from '../../api/hooks/usePeriods'
import EmptyState from '../EmptyState'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import type { SortDir } from './SortControls'
import { ListSkeleton, LoadErrorAlert } from './common'
import EntityAddForm from './EntityAddForm'

export default function YearsTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, isError, refetch } = useYears()
  const create = useCreateYear()
  const [newYear, setNewYear] = useState<number | string>('')
  const [search, setSearch] = useState('')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  const rows = useMemo(() => {
    const filtered = (data ?? []).filter(y => String(y).includes(search))
    return [...filtered].sort((a, b) => sortDir === 'asc' ? a - b : b - a)
  }, [data, search, sortDir])

  const addForm = (
    <EntityAddForm
      opened={addOpened}
      onClose={closeAdd}
      isMobile={isMobile}
      disabled={!newYear}
      loading={create.isPending}
      onSubmit={() => create.mutate(Number(newYear), { onSuccess: () => { setNewYear(''); closeAdd() } })}
    >
      <NumberInput label={t('settings.year')} value={newYear} onChange={setNewYear} hideControls decimalScale={0} />
    </EntityAddForm>
  )

  if (isLoading) return <ListSkeleton />
  if (isError) return <LoadErrorAlert onRetry={() => refetch()} />

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
      {addForm}
    </Stack>
  )
}
