import { useEffect, useRef } from 'react'
import { Modal, Text, Button, Group, Stack, Loader } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useReauth } from '../hooks/useReauth'

interface ReauthConfirmModalProps {
  opened: boolean
  onClose: () => void
  title: string
  warningText: string
  confirmLabel: string
  confirmColor?: string
  loading: boolean
  onConfirm: () => void
}

export default function ReauthConfirmModal({
  opened, onClose, title, warningText, confirmLabel, confirmColor = 'red', loading, onConfirm,
}: ReauthConfirmModalProps) {
  const { t } = useTranslation()
  const { state, start, reset } = useReauth()
  const firedRef = useRef(false)

  useEffect(() => {
    if (state === 'fresh' && !firedRef.current) {
      firedRef.current = true
      onConfirm()
    }
    if (state !== 'fresh') {
      firedRef.current = false
    }
  }, [state, onConfirm])

  const handleClose = () => {
    reset()
    onClose()
  }

  return (
    <Modal opened={opened} onClose={handleClose} title={title}>
      <Stack gap="md">
        <Text size="sm" c="red">{warningText}</Text>

        {state === 'idle' && (
          <>
            <Text size="sm" c="dimmed">{t('common.verifyIdentityHint')}</Text>
            <Group justify="flex-end">
              <Button variant="subtle" onClick={handleClose}>{t('common.cancel')}</Button>
              <Button color={confirmColor} onClick={start}>{t('common.verifyIdentity')}</Button>
            </Group>
          </>
        )}

        {(state === 'pending' || state === 'fresh') && (
          <Group justify="center" py="md">
            <Loader size="sm" />
            <Text size="sm" c="dimmed">
              {state === 'fresh' && loading ? `${confirmLabel}…` : t('common.verifyIdentityPending')}
            </Text>
          </Group>
        )}

        {state === 'cancelled' && (
          <>
            <Text size="sm" c="dimmed">{t('common.verifyIdentityCancelled')}</Text>
            <Group justify="flex-end">
              <Button variant="subtle" onClick={handleClose}>{t('common.cancel')}</Button>
              <Button color={confirmColor} onClick={start}>{t('common.retry')}</Button>
            </Group>
          </>
        )}
      </Stack>
    </Modal>
  )
}
