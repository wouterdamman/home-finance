import { useState } from 'react'
import { Modal, Text, PasswordInput, Button, Group, Stack } from '@mantine/core'
import { useTranslation } from 'react-i18next'

interface PasswordModalProps {
  opened: boolean
  onClose: () => void
  title: string
  warningText: string
  confirmLabel: string
  confirmColor?: string
  loading: boolean
  onConfirm: (password: string) => void
}

export default function PasswordModal({
  opened, onClose, title, warningText, confirmLabel, confirmColor = 'red', loading, onConfirm,
}: PasswordModalProps) {
  const { t } = useTranslation()
  const [password, setPassword] = useState('')

  const handleClose = () => {
    setPassword('')
    onClose()
  }

  const handleConfirm = () => {
    if (!password) return
    onConfirm(password)
    setPassword('')
  }

  return (
    <Modal opened={opened} onClose={handleClose} title={title}>
      <Stack gap="md">
        <Text size="sm" c="red">{warningText}</Text>
        <PasswordInput
          label={t('common.passwordLabel')}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleConfirm()}
          data-autofocus
        />
        <Group justify="flex-end">
          <Button variant="subtle" onClick={handleClose}>{t('common.cancel')}</Button>
          <Button color={confirmColor} loading={loading} disabled={!password} onClick={handleConfirm}>
            {confirmLabel}
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
