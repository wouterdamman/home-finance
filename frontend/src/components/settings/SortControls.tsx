import { Button, Group, Menu, Table } from '@mantine/core'
import { IconArrowsSort, IconChevronDown, IconChevronUp } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'

export type SortDir = 'asc' | 'desc'
export interface SortState<K extends string> { key: K; dir: SortDir }

export function toggleSort<K extends string>(sort: SortState<K>, key: K): SortState<K> {
  if (sort.key !== key) return { key, dir: 'asc' }
  return { key, dir: sort.dir === 'asc' ? 'desc' : 'asc' }
}

export function SortableTh<K extends string>({ label, sortKey, sort, onSort, ta }: {
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

export function cmp(a: number | string, b: number | string): number {
  if (typeof a === 'string' && typeof b === 'string') return a.localeCompare(b)
  return (a as number) - (b as number)
}

export function SortMenu<K extends string>({ sort, onSort, options }: {
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
