import { Center, Card, Stack, Title, Text, Group, Button, Code } from '@mantine/core'
import { IconAlertTriangle } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Link, useRouteError } from 'react-router-dom'

// A redeploy replaces the hashed chunk filenames, so an open tab's lazy()
// import 404s. Nothing but a reload can recover it.
function isChunkLoadError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error ?? '')
  return /dynamically imported module|Importing a module script failed|ChunkLoadError|Loading chunk/i.test(message)
}

export function ErrorCard({
  title,
  message,
  detail,
  actions,
}: {
  title: string
  message: string
  detail?: string
  actions?: React.ReactNode
}) {
  return (
    <Center mih="60vh" p="md">
      <Card shadow="md" p="xl" maw={480} w="100%" withBorder>
        <Stack gap="md">
          <Group gap="xs" wrap="nowrap">
            <IconAlertTriangle size={22} color="var(--mantine-color-red-6)" />
            <Title order={3}>{title}</Title>
          </Group>
          <Text c="dimmed" size="sm">{message}</Text>
          {detail && <Code block>{detail}</Code>}
          {actions && <Group justify="flex-end">{actions}</Group>}
        </Stack>
      </Card>
    </Center>
  )
}

export default function RouteErrorBoundary() {
  const { t } = useTranslation()
  const error = useRouteError()
  const chunkFailure = isChunkLoadError(error)
  const detail = error instanceof Error ? error.message : undefined

  return (
    <ErrorCard
      title={chunkFailure ? t('errors.updateAvailableTitle') : t('errors.boundaryTitle')}
      message={chunkFailure ? t('errors.updateAvailableBody') : t('errors.boundaryBody')}
      detail={chunkFailure ? undefined : detail}
      actions={
        <>
          <Button variant="subtle" onClick={() => window.location.reload()}>{t('errors.reload')}</Button>
          {!chunkFailure && (
            <Button component={Link} to="/" reloadDocument>{t('errors.backToDashboard')}</Button>
          )}
        </>
      }
    />
  )
}
