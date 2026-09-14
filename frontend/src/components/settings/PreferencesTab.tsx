import { Group, SegmentedControl, Select, Stack, Text, useMantineColorScheme } from '@mantine/core'
import { IconDeviceDesktop, IconMoon, IconSun } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'

export default function PreferencesTab() {
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
