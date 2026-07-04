import { Outlet, NavLink as RouterNavLink, useNavigate } from 'react-router-dom'
import { AppShell as MantineAppShell, NavLink, Group, Text, ActionIcon, Select } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'

const currentYear = new Date().getFullYear()
const years = [currentYear - 1, currentYear, currentYear + 1].map(String)

export default function AppShell() {
  const { t, i18n } = useTranslation()
  const { data: user } = useMe()
  const navigate = useNavigate()

  return (
    <MantineAppShell header={{ height: 56 }} navbar={{ width: 220, breakpoint: 'sm' }} padding="md">
      <MantineAppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Text fw={700} size="lg">💰 Home Finance</Text>
          <Group>
            <Select
              size="xs"
              data={['nl', 'en']}
              value={i18n.language.startsWith('nl') ? 'nl' : 'en'}
              onChange={(v) => v && i18n.changeLanguage(v)}
              w={70}
            />
            {user && (
              <ActionIcon variant="subtle" onClick={() => fetch('/auth/logout', { method: 'POST' }).then(() => navigate('/login'))}>
                <Text size="xs">{user.displayName || user.email}</Text>
              </ActionIcon>
            )}
          </Group>
        </Group>
      </MantineAppShell.Header>
      <MantineAppShell.Navbar p="xs">
        {years.map((y) => (
          <NavLink
            key={y}
            label={y}
            component={RouterNavLink}
            to={`/years/${y}`}
          />
        ))}
        <NavLink label={t('pots.title')} component={RouterNavLink} to="/pots" />
        <NavLink label={t('settings.title')} component={RouterNavLink} to="/settings" />
      </MantineAppShell.Navbar>
      <MantineAppShell.Main>
        <Outlet />
      </MantineAppShell.Main>
    </MantineAppShell>
  )
}
