import { useState } from 'react'
import {
  Badge, Button, Checkbox, Code, CopyButton, Group, Modal, Stack, Table, Text, TextInput, Alert,
} from '@mantine/core'
import { modals } from '@mantine/modals'
import { useDisclosure, useMediaQuery } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import { IconAlertTriangle, IconCheck, IconCopy } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import {
  INTEGRATION_SCOPES, useIntegrationTokens, useCreateIntegrationToken, useRevokeIntegrationToken,
} from '../../api/hooks/useIntegrationTokens'
import { getErrorMessage, isUnauthorizedError } from '../../api/client'
import type { IntegrationToken } from '../../api/types'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import ReauthConfirmModal from '../ReauthConfirmModal'
import { ListSkeleton, LoadErrorAlert } from './common'

const NAME_MAX = 100
const DATE_FORMAT = 'DD-MM-YYYY HH:mm'

function NewTokenModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const create = useCreateIntegrationToken()
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<string[]>([])
  const [token, setToken] = useState<string | null>(null)
  const [reauthOpened, { open: openReauth, close: closeReauth }] = useDisclosure(false)

  const trimmed = name.trim()
  const valid = trimmed.length > 0 && trimmed.length <= NAME_MAX && scopes.length > 0

  const handleClose = () => {
    setName('')
    setScopes([])
    setToken(null)
    closeReauth()
    create.reset()
    onClose()
  }

  const submit = () => {
    create.mutate({ name: trimmed, scopes }, {
      onSuccess: (res) => {
        closeReauth()
        setToken(res.token)
      },
      onError: (err: unknown) => {
        if (isUnauthorizedError(err)) {
          openReauth()
          return
        }
        closeReauth()
        notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) })
      },
    })
  }

  return (
    <>
      <Modal
        opened={opened && !reauthOpened}
        onClose={handleClose}
        title={token ? t('settings.integrationsTokenCreatedTitle') : t('settings.integrationsCreateTitle')}
        closeOnClickOutside={!token}
        closeOnEscape={!token}
      >
        {token ? (
          <Stack gap="md">
            <Alert color="orange" icon={<IconAlertTriangle size={16} />}>
              {t('settings.integrationsTokenWarning')}
            </Alert>
            <Group gap="xs" wrap="nowrap" align="flex-end">
              <TextInput
                readOnly
                value={token}
                aria-label={t('settings.integrationsToken')}
                style={{ flex: 1 }}
                styles={{ input: { fontFamily: 'var(--mantine-font-family-monospace)' } }}
                onFocus={(e) => e.currentTarget.select()}
              />
              <CopyButton value={token}>
                {({ copied, copy }) => (
                  <Button
                    variant="light"
                    color={copied ? 'teal' : undefined}
                    leftSection={copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                    onClick={copy}
                  >
                    {copied ? t('settings.integrationsCopied') : t('settings.integrationsCopy')}
                  </Button>
                )}
              </CopyButton>
            </Group>
            <Stack gap={4}>
              <Text size="sm" c="dimmed">{t('settings.integrationsUsage')}</Text>
              <Code block>{`Authorization: Bearer ${token}`}</Code>
              <Code block>/api/integrations/v1/kids/balances</Code>
            </Stack>
            <Group justify="flex-end">
              <Button onClick={handleClose}>{t('settings.integrationsDone')}</Button>
            </Group>
          </Stack>
        ) : (
          <form onSubmit={(e) => { e.preventDefault(); if (valid) submit() }}>
            <Stack gap="md">
              <TextInput
                label={t('settings.integrationsName')}
                description={t('settings.integrationsNameHelp')}
                value={name}
                onChange={(e) => setName(e.currentTarget.value)}
                maxLength={NAME_MAX}
                required
                data-autofocus
              />
              <Checkbox.Group
                label={t('settings.integrationsScopes')}
                value={scopes}
                onChange={setScopes}
                required
              >
                <Stack gap="xs" mt="xs">
                  {INTEGRATION_SCOPES.map(s => (
                    <Checkbox key={s.value} value={s.value} label={t(s.labelKey)} />
                  ))}
                </Stack>
              </Checkbox.Group>
              <Group justify="flex-end">
                <Button variant="subtle" onClick={handleClose}>{t('common.cancel')}</Button>
                <Button type="submit" disabled={!valid} loading={create.isPending && !reauthOpened}>
                  {t('settings.integrationsCreate')}
                </Button>
              </Group>
            </Stack>
          </form>
        )}
      </Modal>
      <ReauthConfirmModal
        opened={opened && reauthOpened}
        onClose={closeReauth}
        title={t('settings.integrationsCreateTitle')}
        warningText={t('settings.integrationsCreateWarning')}
        confirmLabel={t('settings.integrationsCreate')}
        confirmColor="blue"
        loading={create.isPending}
        onConfirm={submit}
      />
    </>
  )
}

export default function IntegrationsTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data: tokens, isLoading, isError, refetch } = useIntegrationTokens()
  const revoke = useRevokeIntegrationToken()
  const [newOpened, { open: openNew, close: closeNew }] = useDisclosure(false)

  const scopeLabel = (scope: string) => {
    const known = INTEGRATION_SCOPES.find(s => s.value === scope)
    return known ? t(known.labelKey) : scope
  }

  const confirmRevoke = (tok: IntegrationToken) => {
    modals.openConfirmModal({
      title: t('settings.integrationsRevokeTitle'),
      children: <Text size="sm">{t('settings.integrationsRevokeConfirm', { name: tok.name })}</Text>,
      labels: { confirm: t('settings.integrationsRevoke'), cancel: t('common.cancel') },
      confirmProps: { color: 'red' },
      onConfirm: () => revoke.mutate(tok.id, {
        onError: (err: unknown) => notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) }),
      }),
    })
  }

  const statusBadge = (tok: IntegrationToken, size?: 'sm') => tok.revokedAt
    ? <Badge color="gray" size={size}>{t('settings.integrationsRevoked')}</Badge>
    : <Badge color="teal" size={size}>{t('settings.integrationsActive')}</Badge>

  const lastUsed = (tok: IntegrationToken) =>
    tok.lastUsedAt ? dayjs(tok.lastUsedAt).format(DATE_FORMAT) : t('settings.integrationsNever')

  const prefix = (tok: IntegrationToken) => (
    <Text span ff="monospace" size="xs">{tok.tokenPrefix}…</Text>
  )

  if (isLoading) return <ListSkeleton />
  if (isError) return <LoadErrorAlert onRetry={() => refetch()} />

  const list = tokens ?? []

  return (
    <Stack gap="md">
      <Group justify="space-between" wrap="nowrap" align="flex-start">
        <Text size="sm" c="dimmed">{t('settings.integrationsIntro')}</Text>
        <Button size="xs" onClick={openNew} style={{ flexShrink: 0 }}>{t('settings.integrationsNewToken')}</Button>
      </Group>

      {list.length === 0 ? (
        <Text c="dimmed" ta="center" py="xl">{t('settings.integrationsEmpty')}</Text>
      ) : isMobile ? (
        <MobileList>
          {list.map(tok => (
            <MobileListRow
              key={tok.id}
              disabled={!!tok.revokedAt}
              title={tok.name}
              subtitle={
                <>
                  {prefix(tok)} · {t('settings.integrationsLastUsed')}: {lastUsed(tok)}
                </>
              }
              trailing={
                <Group gap="xs" wrap="nowrap">
                  {statusBadge(tok, 'sm')}
                  {!tok.revokedAt && (
                    <Button size="compact-xs" variant="light" color="red" onClick={() => confirmRevoke(tok)}>
                      {t('settings.integrationsRevoke')}
                    </Button>
                  )}
                </Group>
              }
            />
          ))}
        </MobileList>
      ) : (
        <Table.ScrollContainer minWidth={760}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('settings.integrationsName')}</Table.Th>
                <Table.Th>{t('settings.integrationsToken')}</Table.Th>
                <Table.Th>{t('settings.integrationsScopes')}</Table.Th>
                <Table.Th>{t('settings.integrationsCreated')}</Table.Th>
                <Table.Th>{t('settings.integrationsLastUsed')}</Table.Th>
                <Table.Th>{t('settings.integrationsStatus')}</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {list.map(tok => (
                <Table.Tr key={tok.id} style={{ opacity: tok.revokedAt ? 0.5 : 1 }}>
                  <Table.Td>{tok.name}</Table.Td>
                  <Table.Td>{prefix(tok)}</Table.Td>
                  <Table.Td>
                    <Group gap={4}>
                      {tok.scopes.map(s => <Badge key={s} variant="light" size="sm">{scopeLabel(s)}</Badge>)}
                    </Group>
                  </Table.Td>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{dayjs(tok.createdAt).format(DATE_FORMAT)}</Table.Td>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{lastUsed(tok)}</Table.Td>
                  <Table.Td>{statusBadge(tok)}</Table.Td>
                  <Table.Td>
                    {!tok.revokedAt && (
                      <Button size="xs" variant="subtle" color="red" loading={revoke.isPending && revoke.variables === tok.id} onClick={() => confirmRevoke(tok)}>
                        {t('settings.integrationsRevoke')}
                      </Button>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}

      <NewTokenModal opened={newOpened} onClose={closeNew} />
    </Stack>
  )
}
