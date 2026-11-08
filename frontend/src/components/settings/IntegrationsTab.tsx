import { useState } from 'react'
import {
  Badge, Button, Checkbox, Code, CopyButton, Group, Modal, Select, Stack, Table, Text, TextInput, Alert,
} from '@mantine/core'
import { modals } from '@mantine/modals'
import { useDisclosure, useMediaQuery } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import { DateTimePicker } from '@mantine/dates'
import { IconAlertTriangle, IconCheck, IconCopy } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import type { ManipulateType } from 'dayjs'
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

// Server bounds live in auth.IntegrationTokenMinTTL / IntegrationTokenMaxExpiry:
// every preset and the custom range must stay inside 1 hour .. 2 years.
const EXPIRY_PRESETS = [
  { value: '1h', amount: 1, unit: 'hour', labelKey: 'settings.integrationsExpiry1h' },
  { value: '1d', amount: 1, unit: 'day', labelKey: 'settings.integrationsExpiry1d' },
  { value: '7d', amount: 7, unit: 'day', labelKey: 'settings.integrationsExpiry7d' },
  { value: '30d', amount: 30, unit: 'day', labelKey: 'settings.integrationsExpiry30d' },
  { value: '90d', amount: 90, unit: 'day', labelKey: 'settings.integrationsExpiry90d' },
  { value: '1y', amount: 1, unit: 'year', labelKey: 'settings.integrationsExpiry1y' },
  { value: '2y', amount: 2, unit: 'year', labelKey: 'settings.integrationsExpiry2y' },
] as const satisfies readonly { value: string; amount: number; unit: ManipulateType; labelKey: string }[]
const EXPIRY_CUSTOM = 'custom'
const EXPIRY_DEFAULT = '1y'

// Evaluated at submit time (not when the modal opens) so a preset like
// "1 hour" is relative to the moment the token is actually created.
function resolveExpiry(preset: string, custom: string | null): dayjs.Dayjs | null {
  const now = dayjs()
  if (preset === EXPIRY_CUSTOM) {
    if (!custom) return null
    const d = dayjs(custom)
    if (!d.isValid() || d.isBefore(now.add(1, 'hour')) || d.isAfter(now.add(2, 'year'))) return null
    return d
  }
  const p = EXPIRY_PRESETS.find(x => x.value === preset)
  return p ? now.add(p.amount, p.unit) : null
}

function NewTokenModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const create = useCreateIntegrationToken()
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<string[]>([])
  const [expiryPreset, setExpiryPreset] = useState<string>(EXPIRY_DEFAULT)
  const [customExpiry, setCustomExpiry] = useState<string | null>(null)
  const [created, setCreated] = useState<{ token: string; expiresAt: string } | null>(null)
  const token = created?.token ?? null
  const [reauthOpened, { open: openReauth, close: closeReauth }] = useDisclosure(false)

  const trimmed = name.trim()
  const customInvalid = expiryPreset === EXPIRY_CUSTOM && !resolveExpiry(EXPIRY_CUSTOM, customExpiry)
  const valid = trimmed.length > 0 && trimmed.length <= NAME_MAX && scopes.length > 0 && !customInvalid

  const handleClose = () => {
    setName('')
    setScopes([])
    setExpiryPreset(EXPIRY_DEFAULT)
    setCustomExpiry(null)
    setCreated(null)
    closeReauth()
    create.reset()
    onClose()
  }

  const submit = () => {
    const expiry = resolveExpiry(expiryPreset, customExpiry)
    if (!expiry) {
      // Only reachable from the reauth retry: a custom moment that was valid
      // when the form was submitted can slip under now+1h while the popup is
      // open. Bring the form back (it shows the range error) instead of
      // leaving the reauth modal waiting with no feedback.
      closeReauth()
      notifications.show({ color: 'red', title: t('common.error'), message: t('settings.integrationsExpiryOutOfRange') })
      return
    }
    // name/scopes/expiry live in this component's state, which survives the
    // reauth round trip (the form is only hidden), so the retry re-sends them.
    create.mutate({ name: trimmed, scopes, expiresAt: expiry.toISOString() }, {
      onSuccess: (res) => {
        closeReauth()
        setCreated({ token: res.token, expiresAt: res.expiresAt })
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
            <Text size="sm">
              {t('settings.integrationsExpiresOn', { date: dayjs(created!.expiresAt).format(DATE_FORMAT) })}
            </Text>
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
              <Select
                label={t('settings.integrationsExpires')}
                data={[
                  ...EXPIRY_PRESETS.map(p => ({ value: p.value, label: t(p.labelKey) })),
                  { value: EXPIRY_CUSTOM, label: t('settings.integrationsExpiryCustom') },
                ]}
                value={expiryPreset}
                onChange={(v) => setExpiryPreset(v ?? EXPIRY_DEFAULT)}
                allowDeselect={false}
                comboboxProps={{ withinPortal: true }}
              />
              {expiryPreset === EXPIRY_CUSTOM && (
                <DateTimePicker
                  label={t('settings.integrationsExpiryCustomLabel')}
                  description={t('settings.integrationsExpiryCustomHelp')}
                  value={customExpiry}
                  onChange={setCustomExpiry}
                  valueFormat={DATE_FORMAT}
                  minDate={dayjs().add(1, 'hour').toDate()}
                  maxDate={dayjs().add(2, 'year').toDate()}
                  error={customExpiry && customInvalid ? t('settings.integrationsExpiryOutOfRange') : undefined}
                  required
                />
              )}
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

  const isExpired = (tok: IntegrationToken) => !tok.revokedAt && !dayjs(tok.expiresAt).isAfter(dayjs())
  const inactive = (tok: IntegrationToken) => !!tok.revokedAt || isExpired(tok)

  const statusBadge = (tok: IntegrationToken, size?: 'sm') => tok.revokedAt
    ? <Badge color="gray" size={size}>{t('settings.integrationsRevoked')}</Badge>
    : isExpired(tok)
      ? <Badge color="orange" size={size}>{t('settings.integrationsExpired')}</Badge>
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
              disabled={inactive(tok)}
              title={tok.name}
              subtitle={
                <>
                  {prefix(tok)} · {t('settings.integrationsLastUsed')}: {lastUsed(tok)} · {t('settings.integrationsExpires')}: {dayjs(tok.expiresAt).format(DATE_FORMAT)}
                </>
              }
              trailing={
                <Group gap="xs" wrap="nowrap">
                  {statusBadge(tok, 'sm')}
                  {!inactive(tok) && (
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
        <Table.ScrollContainer minWidth={860}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('settings.integrationsName')}</Table.Th>
                <Table.Th>{t('settings.integrationsToken')}</Table.Th>
                <Table.Th>{t('settings.integrationsScopes')}</Table.Th>
                <Table.Th>{t('settings.integrationsCreated')}</Table.Th>
                <Table.Th>{t('settings.integrationsLastUsed')}</Table.Th>
                <Table.Th>{t('settings.integrationsExpires')}</Table.Th>
                <Table.Th>{t('settings.integrationsStatus')}</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {list.map(tok => (
                <Table.Tr key={tok.id} style={{ opacity: inactive(tok) ? 0.5 : 1 }}>
                  <Table.Td>{tok.name}</Table.Td>
                  <Table.Td>{prefix(tok)}</Table.Td>
                  <Table.Td>
                    <Group gap={4}>
                      {tok.scopes.map(s => <Badge key={s} variant="light" size="sm">{scopeLabel(s)}</Badge>)}
                    </Group>
                  </Table.Td>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{dayjs(tok.createdAt).format(DATE_FORMAT)}</Table.Td>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{lastUsed(tok)}</Table.Td>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{dayjs(tok.expiresAt).format(DATE_FORMAT)}</Table.Td>
                  <Table.Td>{statusBadge(tok)}</Table.Td>
                  <Table.Td>
                    {!inactive(tok) && (
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
