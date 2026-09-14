import { Button } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { ErrorCard } from '../components/ErrorBoundary'

export default function NotFound() {
  const { t } = useTranslation()
  return (
    <ErrorCard
      title={t('errors.notFoundTitle')}
      message={t('errors.notFoundBody')}
      actions={<Button component={Link} to="/">{t('errors.backToDashboard')}</Button>}
    />
  )
}
