import { useState } from 'react'
import { ActionIcon, Group, Pill, Popover, Stack, Text, TextInput } from '@mantine/core'
import { IconPlus, IconTags } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import {
  useCategoryDescriptionPresets, useCreateCategoryDescriptionPreset, useDeleteCategoryDescriptionPreset,
  useIncomeSourceDescriptionPresets, useCreateIncomeSourceDescriptionPreset, useDeleteIncomeSourceDescriptionPreset,
} from '../../api/hooks/useDescriptionSuggestions'

interface DescriptionPresetLike { id: number; description: string }

// Shared curated-suggestions manager, popover-anchored so it can sit next to
// a category/income-source row's other actions without needing its own
// page or growing the edit form. Controlled so the dropdown — and with it the per-row presets query — only
// mounts once the popover is actually opened; rendering the query in every
// row fired one request per category/income source on tab load.
function PresetsPopover({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const [opened, setOpened] = useState(false)
  return (
    <Popover width={260} withArrow position="bottom-end" shadow="md" opened={opened} onChange={setOpened}>
      <Popover.Target>
        <ActionIcon variant="subtle" size="sm" aria-label={t('settings.manageSuggestions')} onClick={() => setOpened(o => !o)}>
          <IconTags size={14} />
        </ActionIcon>
      </Popover.Target>
      <Popover.Dropdown>{children}</Popover.Dropdown>
    </Popover>
  )
}

function PresetsList({ presets, onAdd, onRemove, adding }: {
  presets: DescriptionPresetLike[]
  onAdd: (description: string) => void
  onRemove: (id: number) => void
  adding: boolean
}) {
  const { t } = useTranslation()
  const [value, setValue] = useState('')
  const submit = () => {
    const trimmed = value.trim()
    if (!trimmed) return
    onAdd(trimmed)
    setValue('')
  }
  return (
    <Stack gap="xs">
      <Text size="xs" fw={600}>{t('settings.suggestions')}</Text>
      {presets.length === 0
        ? <Text size="xs" c="dimmed">{t('settings.noSuggestions')}</Text>
        : (
          <Group gap={4}>
            {presets.map(p => (
              <Pill key={p.id} withRemoveButton onRemove={() => onRemove(p.id)}>{p.description}</Pill>
            ))}
          </Group>
        )}
      <Group gap={4} wrap="nowrap">
        <TextInput
          size="xs"
          placeholder={t('settings.addSuggestion')}
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') submit() }}
          style={{ flex: 1 }}
        />
        <ActionIcon size="sm" variant="filled" loading={adding} disabled={!value.trim()} onClick={submit} aria-label={t('common.add')}>
          <IconPlus size={12} />
        </ActionIcon>
      </Group>
    </Stack>
  )
}

function CategoryPresetsContent({ categoryId }: { categoryId: number }) {
  const { data } = useCategoryDescriptionPresets(categoryId)
  const create = useCreateCategoryDescriptionPreset(categoryId)
  const del = useDeleteCategoryDescriptionPreset(categoryId)
  return <PresetsList presets={data ?? []} onAdd={d => create.mutate(d)} onRemove={id => del.mutate(id)} adding={create.isPending} />
}

export function CategoryPresetsButton({ categoryId }: { categoryId: number }) {
  return <PresetsPopover><CategoryPresetsContent categoryId={categoryId} /></PresetsPopover>
}

function IncomeSourcePresetsContent({ sourceId }: { sourceId: number }) {
  const { data } = useIncomeSourceDescriptionPresets(sourceId)
  const create = useCreateIncomeSourceDescriptionPreset(sourceId)
  const del = useDeleteIncomeSourceDescriptionPreset(sourceId)
  return <PresetsList presets={data ?? []} onAdd={d => create.mutate(d)} onRemove={id => del.mutate(id)} adding={create.isPending} />
}

export function IncomeSourcePresetsButton({ sourceId }: { sourceId: number }) {
  return <PresetsPopover><IncomeSourcePresetsContent sourceId={sourceId} /></PresetsPopover>
}
