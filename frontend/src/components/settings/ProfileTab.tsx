import { useState } from 'react'
import { Avatar, Badge, Button, Divider, FileInput, Group, SegmentedControl, Stack, Text, TextInput } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { IconShieldCheck, IconUpload } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useMe } from '../../api/hooks/useMe'
import { useUpdateMe, useUploadAvatar } from '../../api/hooks/useUsers'
import type { PaletteId } from '../../lib/chartPalette'
import { CHART_PALETTES, PALETTE_IDS } from '../../lib/chartPalette'
import { useChartPalette } from '../../contexts/ChartPaletteContext'

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

const MAX_AVATAR_UPLOAD_BYTES = 10 * 1024 * 1024 // must match backend maxAvatarUploadBytes

export default function ProfileTab() {
  const { t } = useTranslation()
  const { data: me } = useMe()
  const updateMe = useUpdateMe()
  const uploadAvatar = useUploadAvatar()
  const { paletteId, setPaletteId } = useChartPalette()
  // Null means "untouched, show the server's value". Mirroring me.displayName
  // into state through an effect re-seeded it on every ['me'] refetch, wiping
  // what the user had typed (e.g. right after an avatar upload).
  const [editedName, setEditedName] = useState<string | null>(null)
  const name = editedName ?? me?.displayName ?? ''

  const handleAvatarChange = (file: File | null) => {
    if (!file) return
    if (file.size > MAX_AVATAR_UPLOAD_BYTES) {
      notifications.show({ color: 'red', message: t('settings.avatarTooLarge') })
      return
    }
    uploadAvatar.mutate(file)
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
          accept="image/png,image/jpeg"
          onChange={handleAvatarChange}
          loading={uploadAvatar.isPending}
          clearable
        />
      </Group>
      <TextInput label={t('settings.name')} value={name} onChange={e => setEditedName(e.target.value)} />
      <Badge
        w="fit-content"
        color={me?.role === 'admin' ? 'teal' : 'gray'}
        leftSection={<IconShieldCheck size={12} />}
      >
        {me?.role === 'admin' ? t('settings.roleAdmin') : t('settings.roleUser')}
      </Badge>
      <Button onClick={() => updateMe.mutate(name.trim())} loading={updateMe.isPending} disabled={!name.trim()}>
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
