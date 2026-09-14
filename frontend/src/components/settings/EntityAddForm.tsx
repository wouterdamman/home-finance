import { Button, Modal, Stack } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import BottomSheet from '../mobile/BottomSheet'

// Every entity's add form is the same field list shown in a BottomSheet on
// mobile and a Modal on desktop. Rendering it once through this wrapper keeps
// the two forks from drifting apart the way the pot edit handlers did.
export default function EntityAddForm({ opened, onClose, isMobile, disabled, loading, onSubmit, children }: {
  opened: boolean
  onClose: () => void
  isMobile?: boolean
  disabled?: boolean
  loading?: boolean
  onSubmit: () => void
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  const submit = <Button disabled={disabled} loading={loading} onClick={onSubmit}>{t('common.add')}</Button>
  if (isMobile) {
    return (
      <BottomSheet opened={opened} onClose={onClose} title={t('common.addNew')}>
        {children}
        {submit}
      </BottomSheet>
    )
  }
  return (
    <Modal opened={opened} onClose={onClose} title={t('common.addNew')}>
      <Stack gap="sm">
        {children}
        {submit}
      </Stack>
    </Modal>
  )
}
