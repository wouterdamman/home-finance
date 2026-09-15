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

  // The modal stays mounted at every call site (they pass `opened`), so the
  // poll must be torn down whenever it is dismissed — otherwise a popup that
  // completes later fires the destructive action the user just cancelled.
  useEffect(() => {
    if (!opened) {
      firedRef.current = false
      reset()
    }
  }, [opened, reset])

  useEffect(() => {
    if (opened && state === 'fresh' && !firedRef.current) {
      firedRef.current = true
      onConfirm()
    }
    if (state !== 'fresh') {
      firedRef.current = false
    }
  }, [opened, state, onConfirm])

  const handleClose = () => {
    firedRef.current = false
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
