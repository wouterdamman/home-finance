import { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { Button, LoadingOverlay } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useMe } from '../api/hooks/useMe'
import { ErrorCard } from './ErrorBoundary'
import { getErrorMessage } from '../api/client'

export default function AuthGuard({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { data: user, isLoading, isError, error, refetch } = useMe()
  const location = useLocation()

  if (isLoading) return <LoadingOverlay visible />
  // A failed /api/me is an outage, not a logout — bouncing to /login would
  // have people re-entering credentials against a backend that is down.
  if (isError) {
    return (
      <ErrorCard
        title={t('errors.backendUnavailableTitle')}
        message={t('errors.backendUnavailableBody')}
        detail={getErrorMessage(error, t('common.error'))}
        actions={<Button onClick={() => refetch()}>{t('common.retry')}</Button>}
      />
    )
  }
  if (!user) return <Navigate to="/login" state={{ from: location }} replace />
  return <>{children}</>
}
