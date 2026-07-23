import { Paper, Group, Stack, Text, Box } from '@mantine/core'
import { IconChevronRight } from '@tabler/icons-react'
import { Link } from 'react-router-dom'
import { SwipeableList, SwipeableListItem, TrailingActions, SwipeAction, Type as SwipeListType } from 'react-swipeable-list'
import { Children } from 'react'
import type { ReactNode } from 'react'

interface SwipeActionConfig {
  label: ReactNode
  destructive?: boolean
  onTrigger: () => void
}

interface MobileListRowProps {
  leftSection?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  trailing?: ReactNode
  onClick?: () => void
  to?: string
  disabled?: boolean
  danger?: boolean
  chevron?: boolean
  swipeAction?: SwipeActionConfig
}

export function MobileListRow({
  leftSection, title, subtitle, trailing, onClick, to, disabled, danger, chevron, swipeAction,
}: MobileListRowProps) {
  const tappable = !!(onClick || to)
  const linkProps = to ? { component: Link, to } : { component: 'div' as const }
  const content = (
    <Group
      wrap="nowrap"
      justify="space-between"
      align="flex-start"
      py="sm"
      px="md"
      gap="md"
      {...linkProps}
      onClick={disabled ? undefined : onClick}
      style={{
        width: '100%',
        textDecoration: 'none',
        color: 'inherit',
        cursor: tappable && !disabled ? 'pointer' : undefined,
        opacity: disabled ? 0.5 : 1,
        minHeight: 44,
      }}
    >
      <Group wrap="nowrap" gap="sm" style={{ minWidth: 0, flex: 1 }}>
        {leftSection}
        <Stack gap={0} style={{ minWidth: 0 }}>
          <Text fw={600} c={danger ? 'red' : undefined} truncate>{title}</Text>
          {subtitle && <Text component="div" size="xs" c="dimmed" truncate>{subtitle}</Text>}
        </Stack>
      </Group>
      <Group wrap="nowrap" gap={4} style={{ flexShrink: 0 }}>
        {trailing && <Box ta="right" style={{ minWidth: 64 }}>{trailing}</Box>}
        {chevron && tappable && <IconChevronRight size={16} opacity={0.4} />}
      </Group>
    </Group>
  )

  if (!swipeAction) return <Box style={{ borderBottom: '1px solid var(--mantine-color-default-border)' }}>{content}</Box>

  return (
    <SwipeableListItem
      trailingActions={
        <TrailingActions>
          <SwipeAction destructive={swipeAction.destructive} onClick={swipeAction.onTrigger}>
            <Box
              bg={swipeAction.destructive ? 'red' : 'gray'}
              c="white"
              px="md"
              style={{ display: 'flex', alignItems: 'center', height: '100%', whiteSpace: 'nowrap' }}
            >
              {swipeAction.label}
            </Box>
          </SwipeAction>
        </TrailingActions>
      }
    >
      {content}
    </SwipeableListItem>
  )
}

interface MobileListProps {
  children: ReactNode
}

export default function MobileList({ children }: MobileListProps) {
  return (
    <Paper withBorder radius="md" style={{ overflow: 'hidden' }}>
      <SwipeableList type={SwipeListType.IOS} fullSwipe={false}>
        {Children.toArray(children)}
      </SwipeableList>
    </Paper>
  )
}
