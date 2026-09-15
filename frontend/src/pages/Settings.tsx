import { Title, Tabs, Button, Group, Stack, ActionIcon, Divider } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useSearchParams } from 'react-router-dom'
import {
  IconTags, IconCoin, IconPigMoney, IconCalendar, IconChevronLeft, IconPalette,
  IconFileSpreadsheet, IconUsers, IconUserCircle, IconApi, IconHistory,
} from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'
import CategoriesTab from '../components/settings/CategoriesTab'
import SourcesTab from '../components/settings/SourcesTab'
import PotsTab from '../components/settings/PotsTab'
import YearsTab from '../components/settings/YearsTab'
import ExportImportTab from '../components/settings/ExportImportTab'
import ProfileTab from '../components/settings/ProfileTab'
import UsersTab from '../components/settings/UsersTab'
import AuditLogTab from '../components/settings/AuditLogTab'
import PreferencesTab from '../components/settings/PreferencesTab'

type SettingsSection = 'categories' | 'sources' | 'pots' | 'years' | 'exportImport' | 'users' | 'auditLog' | 'apiDocs' | 'profile' | 'preferences'

interface SettingsMenuItem {
  key: SettingsSection
  label: string
  icon: typeof IconTags
  content: React.ReactNode
  adminOnly?: boolean
  external?: string
}

export default function Settings() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const [searchParams, setSearchParams] = useSearchParams()
  const section = searchParams.get('section') as SettingsSection | null
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  // Family-finance admin sections, then sections everyone gets, then the
  // API docs developer resource last — visually separated (Divider/gap)
  // since it's not a family-finance admin task, just a technical reference.
  const allMenuItems: SettingsMenuItem[] = [
    { key: 'categories', label: t('settings.categories'), icon: IconTags, content: <CategoriesTab />, adminOnly: true },
    { key: 'sources', label: t('settings.incomeSources'), icon: IconCoin, content: <SourcesTab />, adminOnly: true },
    { key: 'pots', label: t('settings.pots'), icon: IconPigMoney, content: <PotsTab />, adminOnly: true },
    { key: 'years', label: t('settings.years'), icon: IconCalendar, content: <YearsTab />, adminOnly: true },
    { key: 'exportImport', label: t('settings.exportImport'), icon: IconFileSpreadsheet, content: <ExportImportTab />, adminOnly: true },
    { key: 'users', label: t('settings.users'), icon: IconUsers, content: <UsersTab />, adminOnly: true },
    { key: 'auditLog', label: t('settings.auditLog'), icon: IconHistory, content: <AuditLogTab />, adminOnly: true },
    { key: 'profile', label: t('settings.profile'), icon: IconUserCircle, content: <ProfileTab /> },
    { key: 'preferences', label: t('settings.preferences'), icon: IconPalette, content: <PreferencesTab /> },
    { key: 'apiDocs', label: t('nav.apiDocs'), icon: IconApi, content: null, adminOnly: true, external: '/api/docs' },
  ]
  const menu = allMenuItems.filter(m => isAdmin || !m.adminOnly)

  if (isMobile) {
    const active = menu.find(m => m.key === section)

    if (active) {
      return (
        <Stack gap="md">
          <Group gap="xs" wrap="nowrap">
            <ActionIcon variant="subtle" aria-label={t('common.back')} onClick={() => setSearchParams({})}>
              <IconChevronLeft size={18} />
            </ActionIcon>
            <Title order={3}>{active.label}</Title>
          </Group>
          {active.content}
        </Stack>
      )
    }

    return (
      <Stack gap="md">
        <Title order={2}>{t('settings.title')}</Title>
        <MobileList>
          {menu.filter(m => !m.external).map(m => (
            <MobileListRow
              key={m.key}
              leftSection={<m.icon size={18} />}
              title={m.label}
              chevron
              onClick={() => setSearchParams({ section: m.key })}
            />
          ))}
        </MobileList>
        {menu.filter(m => m.external).map(m => (
          <Stack key={m.key} gap="md">
            <Divider />
            <MobileList>
              <MobileListRow
                leftSection={<m.icon size={18} />}
                title={m.label}
                onClick={() => window.open(m.external, '_blank', 'noopener,noreferrer')}
              />
            </MobileList>
          </Stack>
        ))}
      </Stack>
    )
  }

  const defaultTab = isAdmin ? 'categories' : 'profile'
  const apiDocsItem = menu.find(m => m.external)

  return (
    <>
      <Group justify="space-between" mb="md" wrap="wrap">
        <Title order={2}>{t('settings.title')}</Title>
        {apiDocsItem && (
          <Button
            variant="subtle"
            size="xs"
            color="gray"
            leftSection={<apiDocsItem.icon size={14} />}
            onClick={() => window.open(apiDocsItem.external, '_blank', 'noopener,noreferrer')}
          >
            {apiDocsItem.label}
          </Button>
        )}
      </Group>
      <Tabs
        value={menu.some(m => m.key === section && !m.external) ? section : defaultTab}
        onChange={v => v && setSearchParams({ section: v })}
      >
        <Tabs.List>
          {menu.filter(m => !m.external).map(m => (
            <Tabs.Tab key={m.key} value={m.key}>{m.label}</Tabs.Tab>
          ))}
        </Tabs.List>
        {menu.filter(m => !m.external).map(m => (
          <Tabs.Panel key={m.key} value={m.key} pt="md">{m.content}</Tabs.Panel>
        ))}
      </Tabs>
    </>
  )
}
