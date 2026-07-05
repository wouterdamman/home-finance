import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Title, Table, Text, Button, Group, Skeleton, Alert, Badge, Modal, PasswordInput } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useCreatePeriod, useLockYear, useUnlockYear } from '../api/hooks/usePeriods'
import MoneyText from '../components/MoneyText'

const MONTHS_NL = ['Jan','Feb','Mar','Apr','Mei','Jun','Jul','Aug','Sep','Okt','Nov','Dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

export default function YearDashboard() {
  const { year } = useParams<{ year: string }>()
  const y = Number(year)
  const { t, i18n } = useTranslation()
  const { data, isLoading, error } = useYearSummary(y)
  const createPeriod = useCreatePeriod()
  const lockYear = useLockYear(y)
  const unlockYear = useUnlockYear(y)
  const months = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN

  const [lockModalOpen, setLockModalOpen] = useState(false)
  const [unlockModalOpen, setUnlockModalOpen] = useState(false)
  const [password, setPassword] = useState('')

  if (isLoading) return <Skeleton h={400} />
  if (error) return <Alert color="red">{t('common.error')}</Alert>
  if (!data) return null

  const isLocked = data.locked

  const handleLock = () => {
    lockYear.mutate(password, {
      onSuccess: () => { setLockModalOpen(false); setPassword('') },
      onError: (e: any) => notifications.show({ color: 'red', message: e?.error?.message ?? t('common.error') }),
    })
  }

  const handleUnlock = () => {
    unlockYear.mutate(password, {
      onSuccess: () => { setUnlockModalOpen(false); setPassword('') },
      onError: (e: any) => notifications.show({ color: 'red', message: e?.error?.message ?? t('common.error') }),
    })
  }

  return (
    <>
      <Group justify="space-between" mb="md">
        <Group gap="sm">
          <Title order={2}>{t('year.title', { year: y })}</Title>
          {isLocked && <Badge color="red">{t('year.locked')}</Badge>}
        </Group>
        {isLocked ? (
          <Button size="xs" color="orange" variant="subtle" onClick={() => { setPassword(''); setUnlockModalOpen(true) }}>
            {t('year.unlockAction')}
          </Button>
        ) : (
          <Button size="xs" color="red" variant="subtle" onClick={() => { setPassword(''); setLockModalOpen(true) }}>
            {t('year.lockAction')}
          </Button>
        )}
      </Group>

      <Table striped highlightOnHover mb="xl">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>{t('common.total')}</Table.Th>
            <Table.Th ta="right">{t('year.income')}</Table.Th>
            <Table.Th ta="right">{t('year.expenses')}</Table.Th>
            <Table.Th ta="right">{t('year.surplus')}</Table.Th>
            <Table.Th ta="right">Status</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {data.months.map((m) => (
            <Table.Tr key={m.month}>
              <Table.Td>
                {m.periodId ? (
                  <Text component={Link} to={`/months/${y}/${m.month}`} c="blue">
                    {months[m.month - 1]}
                  </Text>
                ) : (
                  <Group gap="xs">
                    <Text c="dimmed">{months[m.month - 1]}</Text>
                    {!isLocked && (
                      <Button
                        size="compact-xs"
                        variant="subtle"
                        loading={createPeriod.isPending}
                        onClick={() => createPeriod.mutate({ year: y, month: m.month })}
                      >
                        + {t('year.createMonth')}
                      </Button>
                    )}
                  </Group>
                )}
              </Table.Td>
              <Table.Td ta="right"><MoneyText cents={m.incomeTotalCents} /></Table.Td>
              <Table.Td ta="right"><MoneyText cents={m.expenseTotalCents} /></Table.Td>
              <Table.Td ta="right"><MoneyText cents={m.surplusCents} colored /></Table.Td>
              <Table.Td ta="right">
                <Text size="xs" c={m.status === 'closed' ? 'green' : 'orange'}>
                  {m.status ? t(`common.${m.status}`) : '-'}
                </Text>
              </Table.Td>
            </Table.Tr>
          ))}
          <Table.Tr fw={700}>
            <Table.Td>{t('year.yearTotal')}</Table.Td>
            <Table.Td ta="right"><MoneyText cents={data.yearIncomeTotalCents} /></Table.Td>
            <Table.Td ta="right"><MoneyText cents={data.yearExpenseTotalCents} /></Table.Td>
            <Table.Td ta="right"><MoneyText cents={data.yearSurplusCents} colored /></Table.Td>
            <Table.Td />
          </Table.Tr>
        </Table.Tbody>
      </Table>

      <Modal opened={lockModalOpen} onClose={() => setLockModalOpen(false)} title={t('year.lockTitle')}>
        <Text size="sm" mb="md">{t('year.lockWarning', { year: y })}</Text>
        <PasswordInput
          label={t('year.passwordLabel')}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          mb="md"
        />
        <Group justify="flex-end">
          <Button variant="subtle" onClick={() => setLockModalOpen(false)}>{t('common.cancel')}</Button>
          <Button color="red" loading={lockYear.isPending} onClick={handleLock} disabled={!password}>
            {t('year.lockConfirm')}
          </Button>
        </Group>
      </Modal>

      <Modal opened={unlockModalOpen} onClose={() => setUnlockModalOpen(false)} title={t('year.unlockTitle')}>
        <Text size="sm" mb="md">{t('year.unlockWarning', { year: y })}</Text>
        <PasswordInput
          label={t('year.passwordLabel')}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          mb="md"
        />
        <Group justify="flex-end">
          <Button variant="subtle" onClick={() => setUnlockModalOpen(false)}>{t('common.cancel')}</Button>
          <Button color="orange" loading={unlockYear.isPending} onClick={handleUnlock} disabled={!password}>
            {t('year.unlockConfirm')}
          </Button>
        </Group>
      </Modal>
    </>
  )
}
