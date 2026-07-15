import { Outlet, NavLink as RouterNavLink, useNavigate } from 'react-router-dom'
import { AppShell as MantineAppShell, NavLink, Group, Text, ActionIcon, Select, SegmentedControl, useMantineColorScheme } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'
import { useYears } from '../api/hooks/usePeriods'

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
    <MantineAppShell header={{ height: 56 }} navbar={{ width: 220, breakpoint: 'sm' }} padding="md">
      <MantineAppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Text fw={700} size="lg">💰 Home Finance</Text>
          <Group>
            <SegmentedControl
              size="xs"
              value={colorScheme}
              onChange={(v) => setColorScheme(v as 'light' | 'dark' | 'auto')}
              aria-label={t('common.theme')}
              data={[
                { label: <span title={t('common.themeLight')} aria-label={t('common.themeLight')}>☀️</span>, value: 'light' },
                { label: <span title={t('common.themeDark')} aria-label={t('common.themeDark')}>🌙</span>, value: 'dark' },
                { label: <span title={t('common.themeAuto')} aria-label={t('common.themeAuto')}>⚙️</span>, value: 'auto' },
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
              <ActionIcon variant="subtle" aria-label={t('auth.signOut')} title={t('auth.signOut')} onClick={handleLogout}>
                <Text size="xs">{user.displayName || user.email}</Text>
              </ActionIcon>
            )}
          </Group>
        </Group>
      </MantineAppShell.Header>
      <MantineAppShell.Navbar p="xs">
        {(years ?? []).map((y) => (
          <NavLink
            key={y}
            label={String(y)}
            component={RouterNavLink}
            to={`/years/${y}`}
          />
        ))}
        <NavLink label={t('nav.pots')} component={RouterNavLink} to="/pots" />
        <NavLink label={t('settings.title')} component={RouterNavLink} to="/settings" />
      </MantineAppShell.Navbar>
      <MantineAppShell.Main>
        <Outlet />
      </MantineAppShell.Main>
    </MantineAppShell>
  )
}
