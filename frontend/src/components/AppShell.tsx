import { useEffect, useState } from 'react'
import { Outlet, NavLink as RouterNavLink, useLocation } from 'react-router-dom'
import { AppShell as MantineAppShell, NavLink, Group, Text, ActionIcon, Tooltip, Stack, Avatar } from '@mantine/core'
import { IconWallet, IconLogout, IconPigMoney, IconSettings, IconCalendar, IconChevronLeft, IconChevronRight, IconPin, IconPinFilled, IconChartLine, IconUsers } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { useMe } from '../api/hooks/useMe'
import { useYears } from '../api/hooks/usePeriods'
import BottomTabBar from './BottomTabBar'

const NAV_PINNED_KEY = 'nav-pinned'

// Every localStorage key this app writes. Kept here (rather than imported
// from each owner module) so logout can wipe them without pulling the Trends
// dashboard and chart-palette modules into the shell's bundle.
const APP_STORAGE_KEYS = [NAV_PINNED_KEY, 'trends-dashboard-v1', 'trends-year-filter', 'chart-palette']

function safeGetItem(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function safeSetItem(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Safari private mode / quota exceeded — a lost UI preference must never
    // throw out of a click handler.
  }
}

export default function AppShell() {
  const { t } = useTranslation()
  const { data: user } = useMe()
  const { data: years } = useYears()
  const location = useLocation()
  const queryClient = useQueryClient()
  // Sidebar is collapsed (icon rail) by default — it's secondary navigation,
  // most screen width should go to content. "Peek" is a transient expand
  // (click the chevron) that auto-collapses again on the next navigation;
  // "pinned" persists across navigation and reloads via localStorage.
  const [pinned, setPinned] = useState(() => safeGetItem(NAV_PINNED_KEY) === 'true')
  const [peek, setPeek] = useState(false)
  const expanded = pinned || peek

  useEffect(() => {
    setPeek(false)
  }, [location.pathname])

  const pin = () => {
    setPinned(true)
    safeSetItem(NAV_PINNED_KEY, 'true')
  }
  const unpin = () => {
    setPinned(false)
    setPeek(false)
    safeSetItem(NAV_PINNED_KEY, 'false')
  }

  // A client-side navigate() would leave the whole TanStack cache (every
  // period, transaction and the ['me'] record) in memory, so pressing Back
  // repaints the previous user's finances on a shared machine. Purge every
  // client-side store, then hard-navigate so no React state survives.
  const handleLogout = () => {
    const purgeAndLeave = async () => {
      queryClient.clear()
      for (const key of APP_STORAGE_KEYS) {
        try {
          localStorage.removeItem(key)
        } catch {
          // storage unavailable — nothing to purge
        }
      }
      try {
        if ('caches' in window) {
          const names = await caches.keys()
          await Promise.all(names.map((name) => caches.delete(name)))
        }
      } catch {
        // CacheStorage unavailable (or blocked) — nothing to purge
      }
      window.location.assign('/login')
    }
    fetch('/auth/logout', { method: 'POST' }).then(purgeAndLeave, purgeAndLeave)
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
              <Avatar src={user.avatarUrl ?? undefined} size={24} radius="xl">{user.displayName ? user.displayName[0].toUpperCase() : '?'}</Avatar>
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
          {navLink('kids', t('nav.kids'), <IconUsers size={16} />, '/kids')}
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
