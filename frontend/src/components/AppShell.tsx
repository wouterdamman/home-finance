import { Outlet, NavLink as RouterNavLink, useNavigate } from 'react-router-dom'
import { AppShell as MantineAppShell, NavLink, Group, Text, ActionIcon, Select, SegmentedControl, useMantineColorScheme } from '@mantine/core'
import { IconWallet, IconSun, IconMoon, IconDeviceDesktop, IconLogout, IconPigMoney, IconSettings, IconApi, IconCalendar } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'
import { useYears } from '../api/hooks/usePeriods'
import BottomTabBar from './BottomTabBar'

export default function AppShell() {
  const { t, i18n } = useTranslation()
  const { data: user } = useMe()
  const { data: years } = useYears()
  const navigate = useNavigate()
  const { colorScheme, setColorScheme } = useMantineColorScheme()

  const handleLogout = () => {
    fetch('/auth/logout', { method: 'POST' })
      .then(() => navigate('/login'))
      .catch(() => navigate('/login'))
  }

  return (
    <MantineAppShell
      header={{ height: 56 }}
      navbar={{ width: 220, breakpoint: 'sm', collapsed: { mobile: true } }}
      footer={{ height: { base: 60, sm: 0 } }}
      padding="md"
    >
      <MantineAppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="xs" wrap="nowrap">
            <IconWallet size={22} />
            <Text fw={700} size="lg" visibleFrom="xs">Home Finance</Text>
          </Group>
          <Group gap="xs" wrap="nowrap">
            <SegmentedControl
              size="xs"
              value={colorScheme}
              onChange={(v) => setColorScheme(v as 'light' | 'dark' | 'auto')}
              aria-label={t('common.theme')}
              data={[
                { label: <IconSun size={16} title={t('common.themeLight')} aria-label={t('common.themeLight')} />, value: 'light' },
                { label: <IconMoon size={16} title={t('common.themeDark')} aria-label={t('common.themeDark')} />, value: 'dark' },
                { label: <IconDeviceDesktop size={16} title={t('common.themeAuto')} aria-label={t('common.themeAuto')} />, value: 'auto' },
              ]}
            />
            <Select
              size="xs"
              aria-label={t('common.language')}
              data={['nl', 'en']}
              value={i18n.language.startsWith('nl') ? 'nl' : 'en'}
              onChange={(v) => v && i18n.changeLanguage(v)}
              w={70}
            />
            {user && (
              <Group gap={6} wrap="nowrap">
                <Text size="xs" c="dimmed" visibleFrom="sm">{user.displayName || user.email}</Text>
                <ActionIcon variant="subtle" aria-label={t('auth.signOut')} title={t('auth.signOut')} onClick={handleLogout}>
                  <IconLogout size={18} />
                </ActionIcon>
              </Group>
            )}
          </Group>
        </Group>
      </MantineAppShell.Header>
      <MantineAppShell.Navbar p="xs">
        {(years ?? []).map((y) => (
          <NavLink
            key={y}
            label={String(y)}
            leftSection={<IconCalendar size={16} />}
            component={RouterNavLink}
            to={`/years/${y}`}
          />
        ))}
        <NavLink label={t('nav.pots')} leftSection={<IconPigMoney size={16} />} component={RouterNavLink} to="/pots" />
        <NavLink label={t('settings.title')} leftSection={<IconSettings size={16} />} component={RouterNavLink} to="/settings" />
        <NavLink label={t('nav.apiDocs')} leftSection={<IconApi size={16} />} component="a" href="/api/docs" target="_blank" rel="noopener noreferrer" />
      </MantineAppShell.Navbar>
      <MantineAppShell.Main>
        <Outlet />
      </MantineAppShell.Main>
      <MantineAppShell.Footer hiddenFrom="sm">
        <BottomTabBar />
      </MantineAppShell.Footer>
    </MantineAppShell>
  )
}
