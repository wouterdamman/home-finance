import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Title, Table, Text, Button, Group, Skeleton, Alert, Badge, SimpleGrid, Paper, Stack } from '@mantine/core'
import { CompositeChart } from '@mantine/charts'
import { notifications } from '@mantine/notifications'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useCreatePeriod, useLockYear, useUnlockYear } from '../api/hooks/usePeriods'
import MoneyText from '../components/MoneyText'
import PasswordModal from '../components/PasswordModal'
import { getErrorMessage } from '../api/client'
import { formatCents } from '../lib/money'

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

  if (isLoading) return <Skeleton h={400} />
  if (error) return <Alert color="red">{t('common.error')}</Alert>
  if (!data) return null

  const isLocked = data.locked
  const totalPotBalance = data.potBalances.reduce((sum, p) => sum + p.balanceCents, 0)
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  const chartData = data.months.map((m) => ({
    month: months[m.month - 1],
    [t('year.income')]: m.incomeTotalCents / 100,
    [t('year.expenses')]: m.expenseTotalCents / 100,
    [t('year.surplus')]: m.surplusCents / 100,
  }))

  const handleLock = (password: string) => {
    lockYear.mutate(password, {
      onSuccess: () => setLockModalOpen(false),
      onError: (e: unknown) => notifications.show({ color: 'red', message: getErrorMessage(e, t('common.error')) }),
    })
  }

  const handleUnlock = (password: string) => {
    unlockYear.mutate(password, {
      onSuccess: () => setUnlockModalOpen(false),
      onError: (e: unknown) => notifications.show({ color: 'red', message: getErrorMessage(e, t('common.error')) }),
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
          <Button size="xs" color="orange" variant="subtle" onClick={() => setUnlockModalOpen(true)}>
            {t('year.unlockAction')}
          </Button>
        ) : (
          <Button size="xs" color="red" variant="subtle" onClick={() => setLockModalOpen(true)}>
            {t('year.lockAction')}
          </Button>
        )}
      </Group>

      <SimpleGrid cols={{ base: 2, sm: 4 }} mb="lg">
        <Paper shadow="xs" p="md" withBorder>
          <Text size="xs" c="dimmed">{t('year.income')}</Text>
          <MoneyText cents={data.yearIncomeTotalCents} size="lg" fw={700} />
        </Paper>
        <Paper shadow="xs" p="md" withBorder>
          <Text size="xs" c="dimmed">{t('year.expenses')}</Text>
          <MoneyText cents={data.yearExpenseTotalCents} size="lg" fw={700} />
        </Paper>
        <Paper shadow="xs" p="md" withBorder>
          <Text size="xs" c="dimmed">{t('year.surplus')}</Text>
          <MoneyText cents={data.yearSurplusCents} size="lg" fw={700} colored />
        </Paper>
        <Paper shadow="xs" p="md" withBorder>
          <Text size="xs" c="dimmed">{t('pots.balance')}</Text>
          <MoneyText cents={totalPotBalance} size="lg" fw={700} />
        </Paper>
      </SimpleGrid>

      {chartData.some((d) => d[t('year.income')] || d[t('year.expenses')]) && (
        <Paper shadow="xs" p="md" withBorder mb="lg">
          <CompositeChart
            h={260}
            data={chartData}
            dataKey="month"
            valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
            series={[
              { name: t('year.income'), color: 'teal.6', type: 'bar' },
              { name: t('year.expenses'), color: 'red.6', type: 'bar' },
              { name: t('year.surplus'), color: 'blue.6', type: 'line' },
            ]}
          />
        </Paper>
      )}

      {data.potBalances.length > 0 && (
        <SimpleGrid cols={{ base: 2, sm: 3, md: 4 }} mb="lg">
          {data.potBalances.map((p) => (
            <Paper key={p.potId} shadow="xs" p="sm" withBorder>
              <Group justify="space-between" wrap="nowrap">
                <Stack gap={2}>
                  <Text size="sm" fw={600}>{p.name}</Text>
                  <Badge size="xs" variant="light" color={p.kind === 'carryover' ? 'gray' : 'blue'}>
                    {t(`pots.kind_${p.kind}`)}
                  </Badge>
                </Stack>
                <MoneyText cents={p.balanceCents} fw={700} />
              </Group>
            </Paper>
          ))}
        </SimpleGrid>
      )}

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
                {m.status ? (
                  <Badge size="sm" color={m.status === 'closed' ? 'green' : 'orange'}>
                    {t(`common.${m.status}`)}
                  </Badge>
                ) : '-'}
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

      <PasswordModal
        opened={lockModalOpen}
        onClose={() => setLockModalOpen(false)}
        title={t('year.lockTitle')}
        warningText={t('year.lockWarning', { year: y })}
        confirmLabel={t('year.lockConfirm')}
        confirmColor="red"
        loading={lockYear.isPending}
        onConfirm={handleLock}
      />

      <PasswordModal
        opened={unlockModalOpen}
        onClose={() => setUnlockModalOpen(false)}
        title={t('year.unlockTitle')}
        warningText={t('year.unlockWarning', { year: y })}
        confirmLabel={t('year.unlockConfirm')}
        confirmColor="orange"
        loading={unlockYear.isPending}
        onConfirm={handleUnlock}
      />
    </>
  )
}
