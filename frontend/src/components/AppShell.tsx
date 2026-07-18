import { useEffect, useState } from 'react'
import { Outlet, NavLink as RouterNavLink, useLocation, useNavigate } from 'react-router-dom'
import { AppShell as MantineAppShell, NavLink, Group, Text, ActionIcon, Tooltip, Stack } from '@mantine/core'
import { IconWallet, IconLogout, IconPigMoney, IconSettings, IconCalendar, IconChevronLeft, IconChevronRight, IconPin, IconPinFilled, IconChartLine } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'
import { useYears } from '../api/hooks/usePeriods'
import BottomTabBar from './BottomTabBar'

const NAV_PINNED_KEY = 'nav-pinned'

export default function AppShell() {
  const { t } = useTranslation()
  const { data: user } = useMe()
  const { data: years } = useYears()
  const navigate = useNavigate()
  const location = useLocation()
  // Sidebar is collapsed (icon rail) by default — it's secondary navigation,
  // most screen width should go to content. "Peek" is a transient expand
  // (click the chevron) that auto-collapses again on the next navigation;
  // "pinned" persists across navigation and reloads via localStorage.
  const [pinned, setPinned] = useState(() => localStorage.getItem(NAV_PINNED_KEY) === 'true')
  const [peek, setPeek] = useState(false)
  const expanded = pinned || peek

  useEffect(() => {
    setPeek(false)
  }, [location.pathname])

  const pin = () => {
    setPinned(true)
    localStorage.setItem(NAV_PINNED_KEY, 'true')
  }
  const unpin = () => {
    setPinned(false)
    setPeek(false)
    localStorage.setItem(NAV_PINNED_KEY, 'false')
  }

  const handleLogout = () => {
    fetch('/auth/logout', { method: 'POST' })
      .then(() => navigate('/login'))
      .catch(() => navigate('/login'))
  }

  const navLink = (key: string, label: string, icon: React.ReactNode, to: string) => (
    expanded
      ? <NavLink key={key} label={label} leftSection={icon} component={RouterNavLink} to={to} />
      : (
        <Tooltip key={key} label={label} position="right" withArrow>
          <NavLink leftSection={icon} component={RouterNavLink} to={to} aria-label={label} />
        </Tooltip>
      )
  )

  return (
    <MantineAppShell
      header={{ height: 56 }}
      navbar={{ width: expanded ? 220 : 64, breakpoint: 'sm', collapsed: { mobile: true } }}
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
              {/* Desktop logout lives in the sidebar (below); mobile has no
                  sidebar, so it stays here for those viewports. */}
              <ActionIcon hiddenFrom="sm" variant="subtle" aria-label={t('auth.signOut')} title={t('auth.signOut')} onClick={handleLogout}>
                <IconLogout size={18} />
              </ActionIcon>
            </Group>
          )}
        </Group>
      </MantineAppShell.Header>
      <MantineAppShell.Navbar p="xs">
        <Group justify={expanded ? 'flex-end' : 'center'} visibleFrom="sm">
          {!expanded && (
            <Tooltip label={t('nav.expand')} position="right" withArrow>
              <ActionIcon variant="subtle" color="gray" size="sm" onClick={() => setPeek(true)} aria-label={t('nav.expand')}>
                <IconChevronRight size={16} />
              </ActionIcon>
            </Tooltip>
          )}
          {expanded && !pinned && (
            <Tooltip label={t('nav.pin')} position="right" withArrow>
              <ActionIcon variant="subtle" color="gray" size="sm" onClick={pin} aria-label={t('nav.pin')}>
                <IconPin size={16} />
              </ActionIcon>
            </Tooltip>
          )}
          {expanded && !pinned && (
            <Tooltip label={t('nav.collapse')} position="right" withArrow>
              <ActionIcon variant="subtle" color="gray" size="sm" onClick={() => setPeek(false)} aria-label={t('nav.collapse')}>
                <IconChevronLeft size={16} />
              </ActionIcon>
            </Tooltip>
          )}
          {pinned && (
            <Tooltip label={t('nav.unpin')} position="right" withArrow>
              <ActionIcon variant="subtle" color="teal" size="sm" onClick={unpin} aria-label={t('nav.unpin')}>
                <IconPinFilled size={16} />
              </ActionIcon>
            </Tooltip>
          )}
        </Group>
        <Stack gap={2} h="100%">
          {(years ?? []).map((y) => navLink(String(y), String(y), <IconCalendar size={16} />, `/years/${y}`))}
          {navLink('pots', t('nav.pots'), <IconPigMoney size={16} />, '/pots')}
          {navLink('trends', t('nav.trends'), <IconChartLine size={16} />, '/trends')}
          {navLink('settings', t('settings.title'), <IconSettings size={16} />, '/settings')}
          <Stack gap={2} mt="auto">
            {expanded ? (
              <NavLink label={t('auth.signOut')} leftSection={<IconLogout size={16} />} onClick={handleLogout} />
            ) : (
              <Tooltip label={t('auth.signOut')} position="right" withArrow>
                <NavLink leftSection={<IconLogout size={16} />} onClick={handleLogout} aria-label={t('auth.signOut')} />
              </Tooltip>
            )}
          </Stack>
        </Stack>
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
