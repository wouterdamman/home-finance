import { Outlet, NavLink as RouterNavLink, useNavigate } from 'react-router-dom'
import { AppShell as MantineAppShell, NavLink, Group, Text, ActionIcon } from '@mantine/core'
import { IconWallet, IconLogout, IconPigMoney, IconSettings, IconApi, IconCalendar } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'
import { useYears } from '../api/hooks/usePeriods'
import BottomTabBar from './BottomTabBar'

export default function AppShell() {
  const { t } = useTranslation()
  const { data: user } = useMe()
  const { data: years } = useYears()
  const navigate = useNavigate()

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
          {user && (
            <Group gap={6} wrap="nowrap">
              <Text size="xs" c="dimmed" visibleFrom="sm">{user.displayName || user.email}</Text>
              <ActionIcon variant="subtle" aria-label={t('auth.signOut')} title={t('auth.signOut')} onClick={handleLogout}>
                <IconLogout size={18} />
              </ActionIcon>
            </Group>
          )}
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
