import { useState } from 'react'
import { Badge, Button, Group, Stack, Table, Text, TextInput } from '@mantine/core'
import { DateInput } from '@mantine/dates'
import { useDebouncedValue, useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { useAuditLog } from '../../api/hooks/useAuditLog'
import EmptyState from '../EmptyState'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import { ListSkeleton, LoadErrorAlert } from './common'

export default function AuditLogTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const [action, setAction] = useState('')
  const [entityType, setEntityType] = useState('')
  const [userEmail, setUserEmail] = useState('')
  const [from, setFrom] = useState<string | null>(null)
  const [to, setTo] = useState<string | null>(null)

  // Debounce the free-text filters so typing "period.close" doesn't fire a
  // request per keystroke — exact-match filters can only hit on the final
  // string anyway.
  const [debouncedAction] = useDebouncedValue(action, 300)
  const [debouncedEntityType] = useDebouncedValue(entityType, 300)
  const [debouncedUserEmail] = useDebouncedValue(userEmail, 300)

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading, isError, refetch } = useAuditLog({
    action: debouncedAction, entityType: debouncedEntityType, userEmail: debouncedUserEmail, from, to,
  })
  const entries = data?.pages.flat() ?? []

  const entityLabel = (e: { entityType?: string; entityId?: number }) =>
    e.entityType ? `${e.entityType}${e.entityId != null ? ' #' + e.entityId : ''}` : '—'

  return (
    <Stack gap="sm">
      <Group wrap="wrap" gap="sm">
        <TextInput label={t('settings.auditLogAction')} value={action} onChange={e => setAction(e.target.value)} placeholder="period.close" w={180} />
        <TextInput label={t('settings.auditLogEntityType')} value={entityType} onChange={e => setEntityType(e.target.value)} placeholder="period" w={140} />
        <TextInput label={t('settings.auditLogUser')} value={userEmail} onChange={e => setUserEmail(e.target.value)} w={200} />
        <DateInput label={t('settings.auditLogFrom')} value={from} onChange={setFrom} valueFormat="DD-MM-YYYY" clearable w={140} />
        <DateInput label={t('settings.auditLogTo')} value={to} onChange={setTo} valueFormat="DD-MM-YYYY" clearable w={140} />
      </Group>

      {isError ? (
        <LoadErrorAlert onRetry={() => refetch()} />
      ) : isLoading ? (
        <ListSkeleton />
      ) : entries.length === 0 ? (
        <EmptyState message={t('settings.auditLogEmpty')} />
      ) : isMobile ? (
        <MobileList>
          {entries.map(e => (
            <MobileListRow
              key={e.id}
              title={e.action}
              subtitle={`${dayjs(e.createdAt).format('DD-MM-YYYY HH:mm')} · ${e.userEmail ?? '—'}`}
              trailing={<Badge size="xs" color="gray">{entityLabel(e)}</Badge>}
            />
          ))}
        </MobileList>
      ) : (
        <Table.ScrollContainer minWidth={700}>
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t('settings.auditLogDate')}</Table.Th>
                <Table.Th>{t('settings.auditLogUser')}</Table.Th>
                <Table.Th>{t('settings.auditLogAction')}</Table.Th>
                <Table.Th>{t('settings.auditLogEntityType')}</Table.Th>
                <Table.Th>{t('settings.auditLogDetails')}</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {entries.map(e => (
                <Table.Tr key={e.id}>
                  <Table.Td style={{ whiteSpace: 'nowrap' }}>{dayjs(e.createdAt).format('DD-MM-YYYY HH:mm')}</Table.Td>
                  <Table.Td>{e.userEmail ?? '—'}</Table.Td>
                  <Table.Td>{e.action}</Table.Td>
                  <Table.Td>{entityLabel(e)}</Table.Td>
                  <Table.Td><Text size="xs" c="dimmed" maw={300} style={{ whiteSpace: 'pre-wrap' }}>{e.details ?? ''}</Text></Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}

      {hasNextPage && (
        <Button variant="subtle" size="xs" onClick={() => fetchNextPage()} loading={isFetchingNextPage}>
          {t('settings.auditLogLoadMore')}
        </Button>
      )}
    </Stack>
  )
}
