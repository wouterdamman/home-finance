import { Stack, Text, ThemeIcon } from '@mantine/core'
import { IconInbox } from '@tabler/icons-react'
import type { ReactNode } from 'react'

interface Props {
  message: string
  icon?: ReactNode
}

export default function EmptyState({ message, icon }: Props) {
  return (
    <Stack align="center" gap="xs" py="xl">
      <ThemeIcon size={40} variant="light" color="gray" radius="xl">
        {icon ?? <IconInbox size={22} />}
      </ThemeIcon>
      <Text c="dimmed" size="sm">{message}</Text>
    </Stack>
  )
}
