import { Link, useLocation } from 'react-router-dom'
import { Group, Stack, Text } from '@mantine/core'
import { IconHome, IconChartLine, IconPigMoney, IconUsers, IconSettings } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useCurrentYear } from '../api/hooks/useCurrentYear'

interface Tab {
  key: string
  label: string
  icon: typeof IconHome
  to: string
  isActive: (pathname: string) => boolean
}

export default function BottomTabBar() {
  const { t } = useTranslation()
  const location = useLocation()
  const currentYear = useCurrentYear()

  const tabs: Tab[] = [
    {
      key: 'home',
      label: t('nav.home'),
      icon: IconHome,
      to: `/years/${currentYear}`,
      isActive: (p) => p.startsWith('/years/') || p.startsWith('/months/'),
    },
    {
      key: 'pots',
      label: t('nav.pots'),
      icon: IconPigMoney,
      to: '/pots',
      isActive: (p) => p.startsWith('/pots'),
    },
    {
      key: 'trends',
      label: t('nav.trends'),
      icon: IconChartLine,
      to: '/trends',
      isActive: (p) => p.startsWith('/trends'),
    },
    {
      key: 'kids',
      label: t('nav.kids'),
      icon: IconUsers,
      to: '/kids',
      isActive: (p) => p.startsWith('/kids'),
    },
    {
      key: 'settings',
      label: t('settings.title'),
      icon: IconSettings,
      to: '/settings',
      isActive: (p) => p.startsWith('/settings'),
    },
  ]

  const activeKey = tabs.find((tab) => tab.isActive(location.pathname))?.key

  return (
    <Group h="100%" grow gap={0} px="xs" style={{ paddingBottom: 'env(safe-area-inset-bottom)' }}>
      {tabs.map((tab) => {
        const Icon = tab.icon
        const active = tab.key === activeKey
        const linkProps = { component: Link, to: tab.to }
        return (
          <Stack
            key={tab.key}
            {...linkProps}
            align="center"
            justify="center"
            gap={2}
            py={6}
            style={{ textDecoration: 'none', color: active ? 'var(--mantine-color-teal-6)' : 'var(--mantine-color-dimmed)' }}
          >
            <Icon size={22} stroke={active ? 2.2 : 1.8} />
            <Text fz={10} fw={active ? 600 : 400} style={{ whiteSpace: 'nowrap' }}>{tab.label}</Text>
          </Stack>
        )
      })}
    </Group>
  )
}
