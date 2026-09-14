import { Alert, Button, Skeleton, Stack, Text } from '@mantine/core'
import { IconAlertTriangle } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { parseToCents } from '../../lib/money'

export function amountToCents(v: number | string): number {
  return parseToCents(String(v)) ?? 0
}

export function ListSkeleton() {
  return (
    <Stack gap="sm">
      <Skeleton h={36} />
      <Skeleton h={28} />
      <Skeleton h={28} />
      <Skeleton h={28} />
    </Stack>
  )
}

// Without this every tab renders its "nothing here yet" empty state on a
// failed fetch, which reads as "your data is gone" rather than "the request
// failed".
export function LoadErrorAlert({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <Alert color="red" icon={<IconAlertTriangle size={16} />} title={t('common.error')}>
      <Stack gap="xs" align="flex-start">
        <Text size="sm">{t('common.loadFailed')}</Text>
        <Button size="xs" variant="light" color="red" onClick={onRetry}>{t('common.retry')}</Button>
      </Stack>
    </Alert>
  )
}
