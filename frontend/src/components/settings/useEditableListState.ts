import { useMemo, useRef, useState } from 'react'
import { useDisclosure } from '@mantine/hooks'
import type { SortState } from './SortControls'
import { toggleSort } from './SortControls'

// Every editable list tab in Settings repeats the same quartet of list state
// plus a filter-then-sort memo over the fetched rows; only the predicates and
// the optional post-sort ordering (the category hierarchy) differ.
export function useEditableListState<T, K extends string>({ data, initialSort, matches, compare, order }: {
  data: T[] | undefined
  initialSort: SortState<K>
  matches: (item: T, search: string) => boolean
  compare: (a: T, b: T, key: K) => number
  order?: (sorted: T[]) => T[]
}) {
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState<SortState<K>>(initialSort)
  const [editing, setEditing] = useState<number | null>(null)
  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)

  // The predicates are inline arrows at every call site, so memoising on them
  // directly would recompute on every render — they're pure functions of the
  // memo's own inputs, so the latest ones are always the right ones to use.
  const predicates = useRef({ matches, compare, order })
  predicates.current = { matches, compare, order }

  const rows = useMemo(() => {
    const { matches: m, compare: c, order: o } = predicates.current
    const sorted = (data ?? []).filter(item => m(item, search)).sort((a, b) => {
      const r = c(a, b, sort.key)
      return sort.dir === 'asc' ? r : -r
    })
    return o ? o(sorted) : sorted
  }, [data, search, sort])

  const onSort = (key: K) => setSort(s => toggleSort(s, key))

  return { search, setSearch, sort, onSort, editing, setEditing, addOpened, openAdd, closeAdd, rows }
}
