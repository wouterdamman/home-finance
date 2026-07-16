import { Drawer, Stack, Box } from '@mantine/core'
import type { ReactNode } from 'react'

interface Props {
  opened: boolean
  onClose: () => void
  title?: ReactNode
  children: ReactNode
}

// Universal replacement for inline table-cell editing and desktop `Modal`
// forms on mobile: same controlled inputs/mutations, just slides up from
// the bottom like an iOS action sheet instead of centering like a dialog.
export default function BottomSheet({ opened, onClose, title, children }: Props) {
  return (
    <Drawer
      opened={opened}
      onClose={onClose}
      title={title}
      position="bottom"
      padding="md"
      styles={{
        content: {
          borderTopLeftRadius: 'var(--mantine-radius-lg)',
          borderTopRightRadius: 'var(--mantine-radius-lg)',
          maxHeight: '85vh',
          height: 'auto',
        },
      }}
    >
      <Box w={36} h={4} bg="var(--mantine-color-gray-4)" style={{ borderRadius: 2 }} mx="auto" mb="sm" />
      <Stack gap="sm" pb="env(safe-area-inset-bottom)">
        {children}
      </Stack>
    </Drawer>
  )
}
