import { Badge, Button, Group, Table, Text } from '@mantine/core'
import { modals } from '@mantine/modals'
import { useMediaQuery } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import { useMe } from '../../api/hooks/useMe'
import { useUsers, useUpdateUserRole } from '../../api/hooks/useUsers'
import { getErrorMessage } from '../../api/client'
import MobileList, { MobileListRow } from '../mobile/MobileList'
import { ListSkeleton, LoadErrorAlert } from './common'

export default function UsersTab() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data: me } = useMe()
  const { data: users, isLoading, isError, refetch } = useUsers()
  const updateRole = useUpdateUserRole()

  const roleLabel = (role: string) => role === 'admin' ? t('settings.roleAdmin') : t('settings.roleUser')

  const toggleRole = (u: { id: number; role: string; displayName: string; email: string }) => {
    const nextRole = u.role === 'admin' ? 'user' : 'admin'
    const applyChange = () => updateRole.mutate({ id: u.id, role: nextRole }, {
      onError: (err: unknown) => notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) }),
    })
    if (nextRole === 'user' && u.id === me?.id) {
      modals.openConfirmModal({
        title: t('settings.demoteSelfTitle'),
        children: <Text size="sm">{t('settings.demoteSelfConfirm')}</Text>,
        labels: { confirm: t('common.confirm'), cancel: t('common.cancel') },
        confirmProps: { color: 'red' },
        onConfirm: applyChange,
      })
      return
    }
    if (nextRole === 'admin') {
      modals.openConfirmModal({
        title: t('settings.promoteTitle'),
        children: <Text size="sm">{t('settings.promoteConfirm', { name: u.displayName || u.email })}</Text>,
        labels: { confirm: t('common.confirm'), cancel: t('common.cancel') },
        onConfirm: applyChange,
      })
      return
    }
    applyChange()
  }

  if (isLoading) return <ListSkeleton />
  if (isError) return <LoadErrorAlert onRetry={() => refetch()} />

  if (isMobile) {
    return (
      <MobileList>
        {(users ?? []).map(u => (
          <MobileListRow
            key={u.id}
            title={u.displayName || u.email}
            subtitle={u.email}
            trailing={
              <Group gap="xs" wrap="nowrap">
                <Badge color={u.role === 'admin' ? 'teal' : 'gray'} size="sm">{roleLabel(u.role)}</Badge>
                <Button size="compact-xs" variant="light" loading={updateRole.isPending} onClick={() => toggleRole(u)}>
                  {u.role === 'admin' ? t('settings.demote') : t('settings.promote')}
                </Button>
              </Group>
            }
          />
        ))}
      </MobileList>
    )
  }

  return (
    <Table.ScrollContainer minWidth={480}>
      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('settings.name')}</Table.Th>
            <Table.Th>{t('settings.email')}</Table.Th>
            <Table.Th>{t('settings.role')}</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {(users ?? []).map(u => (
            <Table.Tr key={u.id}>
              <Table.Td>{u.displayName || '—'}</Table.Td>
              <Table.Td>{u.email}</Table.Td>
              <Table.Td><Badge color={u.role === 'admin' ? 'teal' : 'gray'}>{roleLabel(u.role)}</Badge></Table.Td>
              <Table.Td>
                <Button size="xs" variant="subtle" loading={updateRole.isPending} onClick={() => toggleRole(u)}>
                  {u.role === 'admin' ? t('settings.demote') : t('settings.promote')}
                </Button>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  )
}
