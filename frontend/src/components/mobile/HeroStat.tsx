import { Stack, Text, Group } from '@mantine/core'
import type { ReactNode } from 'react'

interface Props {
  label: string
  value: ReactNode
  delta?: { value: string; positive: boolean }
  size?: 'lg' | 'xl'
}

export default function HeroStat({ label, value, delta, size = 'xl' }: Props) {
  return (
    <Stack gap={2} align="center" py="sm">
      <Text size="sm" c="dimmed">{label}</Text>
      <Text fw={800} size={size === 'xl' ? '2.5rem' : '2rem'} lh={1.1}>{value}</Text>
      {delta && (
        <Group gap={4}>
          <Text size="sm" c={delta.positive ? 'green' : 'red'} fw={600}>{delta.value}</Text>
        </Group>
      )}
    </Stack>
  )
}
