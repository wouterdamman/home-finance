import { useState } from 'react'
import { Title, Tabs, Table, Button, Group, TextInput, NumberInput, Switch, Stack, Badge } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import {
  useCategories, useCreateCategory, useUpdateCategory, useArchiveCategory,
  useIncomeSources, useCreateIncomeSource, useUpdateIncomeSource, useArchiveIncomeSource,
  usePots, useCreatePot, useUpdatePot, useArchivePot,
} from '../api/hooks/useSettings'

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
        </Tabs.List>
        <Tabs.Panel value="categories" pt="md"><CategoriesTab /></Tabs.Panel>
        <Tabs.Panel value="sources" pt="md"><SourcesTab /></Tabs.Panel>
        <Tabs.Panel value="pots" pt="md"><PotsTab /></Tabs.Panel>
      </Tabs>
    </>
  )
}

function CategoriesTab() {
  const { data } = useCategories()
  const create = useCreateCategory()
  const update = useUpdateCategory()
  const archive = useArchiveCategory()
  const [editing, setEditing] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [editItemized, setEditItemized] = useState(false)
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')
  const [newItemized, setNewItemized] = useState(false)

  const startEdit = (cat: { id: number; name: string; defaultAmountCents: number; isItemized: boolean }) => {
    setEditing(cat.id)
    setEditName(cat.name)
    setEditAmount(cat.defaultAmountCents / 100)
    setEditItemized(cat.isItemized)
  }

  const saveEdit = () => {
    if (!editing) return
    update.mutate({ id: editing, name: editName, defaultAmountCents: Math.round(Number(editAmount) * 100), isItemized: editItemized }, {
      onSuccess: () => setEditing(null)
    })
  }

  return (
    <Stack gap="sm">
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Naam</Table.Th>
            <Table.Th ta="right">Standaard</Table.Th>
            <Table.Th>Itemized</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(data ?? []).map(cat => (
            <Table.Tr key={cat.id} opacity={cat.archivedAt ? 0.5 : 1}>
              {editing === cat.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Table.Td>
                    <Table.Td><Switch checked={editItemized} onChange={e => setEditItemized(e.target.checked)} /></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={saveEdit}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}>✕</Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{cat.name} {cat.archivedAt && <Badge size="xs" color="gray">gearchiveerd</Badge>}</Table.Td>
                    <Table.Td ta="right">€ {(cat.defaultAmountCents / 100).toFixed(2)}</Table.Td>
                    <Table.Td>{cat.isItemized ? '✓' : ''}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => startEdit(cat)}>Bewerk</Button>
                        <Button size="xs" variant="subtle" color={cat.archivedAt ? 'green' : 'red'} onClick={() => archive.mutate(cat.id)}>
                          {cat.archivedAt ? 'Herstel' : 'Archiveer'}
                        </Button>
                      </Group>
                    </Table.Td>
                  </>
              }
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Group gap="xs" align="flex-end">
        <TextInput placeholder="Naam" value={newName} onChange={e => setNewName(e.target.value)} size="sm" />
        <NumberInput placeholder="Standaard" value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls size="sm" w={140} />
        <Switch label="Itemized" checked={newItemized} onChange={e => setNewItemized(e.target.checked)} />
        <Button size="sm" disabled={!newName} loading={create.isPending} onClick={() => {
          create.mutate({ name: newName, defaultAmountCents: Math.round(Number(newAmount) * 100), isItemized: newItemized, sortOrder: (data?.length ?? 0) }, {
            onSuccess: () => { setNewName(''); setNewAmount(''); setNewItemized(false) }
          })
        }}>Toevoegen</Button>
      </Group>
    </Stack>
  )
}

function SourcesTab() {
  const { data } = useIncomeSources()
  const create = useCreateIncomeSource()
  const update = useUpdateIncomeSource()
  const archive = useArchiveIncomeSource()
  const [editing, setEditing] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editAmount, setEditAmount] = useState<number | string>('')
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState<number | string>('')

  return (
    <Stack gap="sm">
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Naam</Table.Th>
            <Table.Th ta="right">Standaard</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(data ?? []).map(src => (
            <Table.Tr key={src.id} opacity={src.archivedAt ? 0.5 : 1}>
              {editing === src.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td><NumberInput size="xs" value={editAmount} onChange={setEditAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls w={120} /></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={() => update.mutate({ id: src.id, name: editName, defaultAmountCents: Math.round(Number(editAmount) * 100), sortOrder: src.sortOrder }, { onSuccess: () => setEditing(null) })}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}>✕</Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{src.name} {src.archivedAt && <Badge size="xs" color="gray">gearchiveerd</Badge>}</Table.Td>
                    <Table.Td ta="right">€ {(src.defaultAmountCents / 100).toFixed(2)}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => { setEditing(src.id); setEditName(src.name); setEditAmount(src.defaultAmountCents / 100) }}>Bewerk</Button>
                        <Button size="xs" variant="subtle" color={src.archivedAt ? 'green' : 'red'} onClick={() => archive.mutate(src.id)}>
                          {src.archivedAt ? 'Herstel' : 'Archiveer'}
                        </Button>
                      </Group>
                    </Table.Td>
                  </>
              }
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Group gap="xs" align="flex-end">
        <TextInput placeholder="Naam" value={newName} onChange={e => setNewName(e.target.value)} size="sm" />
        <NumberInput placeholder="Standaard" value={newAmount} onChange={setNewAmount} decimalSeparator="," decimalScale={2} prefix="€ " hideControls size="sm" w={140} />
        <Button size="sm" disabled={!newName} loading={create.isPending} onClick={() => {
          create.mutate({ name: newName, defaultAmountCents: Math.round(Number(newAmount) * 100), sortOrder: (data?.length ?? 0) }, {
            onSuccess: () => { setNewName(''); setNewAmount('') }
          })
        }}>Toevoegen</Button>
      </Group>
    </Stack>
  )
}

function PotsTab() {
  const { data } = usePots()
  const create = useCreatePot()
  const update = useUpdatePot()
  const archive = useArchivePot()
  const [editing, setEditing] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [newName, setNewName] = useState('')
  const newKind = 'normal'

  return (
    <Stack gap="sm">
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Naam</Table.Th>
            <Table.Th>Type</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(data ?? []).map(pot => (
            <Table.Tr key={pot.id} opacity={pot.archivedAt ? 0.5 : 1}>
              {editing === pot.id
                ? <>
                    <Table.Td><TextInput size="xs" value={editName} onChange={e => setEditName(e.target.value)} /></Table.Td>
                    <Table.Td>{pot.kind}</Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" onClick={() => update.mutate({ id: pot.id, name: editName, sortOrder: pot.sortOrder }, { onSuccess: () => setEditing(null) })}>OK</Button>
                        <Button size="xs" variant="subtle" onClick={() => setEditing(null)}>✕</Button>
                      </Group>
                    </Table.Td>
                  </>
                : <>
                    <Table.Td>{pot.name} {pot.archivedAt && <Badge size="xs" color="gray">gearchiveerd</Badge>}</Table.Td>
                    <Table.Td><Badge size="xs" color={pot.kind === 'carryover' ? 'blue' : 'gray'}>{pot.kind}</Badge></Table.Td>
                    <Table.Td>
                      <Group gap="xs">
                        <Button size="xs" variant="subtle" onClick={() => { setEditing(pot.id); setEditName(pot.name) }}>Bewerk</Button>
                        <Button size="xs" variant="subtle" color={pot.archivedAt ? 'green' : 'red'} onClick={() => archive.mutate(pot.id)}>
                          {pot.archivedAt ? 'Herstel' : 'Archiveer'}
                        </Button>
                      </Group>
                    </Table.Td>
                  </>
              }
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      <Group gap="xs" align="flex-end">
        <TextInput placeholder="Naam" value={newName} onChange={e => setNewName(e.target.value)} size="sm" />
        <Button size="sm" disabled={!newName} loading={create.isPending} onClick={() => {
          create.mutate({ name: newName, kind: newKind, sortOrder: (data?.length ?? 0) }, {
            onSuccess: () => setNewName('')
          })
        }}>Toevoegen (normaal)</Button>
      </Group>
    </Stack>
  )
}
