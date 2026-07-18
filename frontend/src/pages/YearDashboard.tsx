import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { Title, Table, Text, Button, Group, Skeleton, Alert, Badge, SimpleGrid, Paper, ActionIcon, ScrollArea, Stack } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { CompositeChart } from '@mantine/charts'
import { notifications } from '@mantine/notifications'
import { IconChevronLeft, IconChevronRight, IconChartLine } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useYearSummary, useCreatePeriod, useLockYear, useUnlockYear, useYears } from '../api/hooks/usePeriods'
import { useMe } from '../api/hooks/useMe'
import MoneyText from '../components/MoneyText'
import PasswordModal from '../components/PasswordModal'
import { getErrorMessage } from '../api/client'
import { formatCents } from '../lib/money'
import HeroStat from '../components/mobile/HeroStat'
import MobileList, { MobileListRow } from '../components/mobile/MobileList'

const MONTHS_NL = ['Jan','Feb','Mar','Apr','Mei','Jun','Jul','Aug','Sep','Okt','Nov','Dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

export default function YearDashboard() {
  const { year } = useParams<{ year: string }>()
  const y = Number(year)
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, error } = useYearSummary(y)
  const { data: years } = useYears()
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'
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
  const savingsPotBalances = data.potBalances.filter((p) => p.kind !== 'carryover')
  const hasPrevYear = (years ?? []).includes(y - 1)
  const hasNextYear = (years ?? []).includes(y + 1)

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

  if (isMobile) {
    return (
      <Stack gap="md">
        <Group justify="space-between" wrap="nowrap">
          <ActionIcon variant="subtle" disabled={!hasPrevYear} aria-label={t('year.prevYear')} onClick={() => navigate(`/years/${y - 1}`)}>
            <IconChevronLeft size={18} />
          </ActionIcon>
          <Group gap="xs">
            <Title order={3}>{y}</Title>
            {isLocked && <Badge color="red">{t('year.locked')}</Badge>}
          </Group>
          <Group gap={4} wrap="nowrap">
            <ActionIcon variant="subtle" aria-label={t('year.trendsLink')} title={t('year.trendsLink')} onClick={() => navigate(`/years/${y}/trends`)}>
              <IconChartLine size={18} />
            </ActionIcon>
            <ActionIcon variant="subtle" disabled={!hasNextYear} aria-label={t('year.nextYear')} onClick={() => navigate(`/years/${y + 1}`)}>
              <IconChevronRight size={18} />
            </ActionIcon>
          </Group>
        </Group>

        <HeroStat label={t('year.surplus')} value={<MoneyText cents={data.yearSurplusCents} span fw={800} size="2.5rem" colored />} />

        <SimpleGrid cols={3}>
          <Stack gap={0} align="center">
            <Text size="xs" c="dimmed">{t('year.income')}</Text>
            <MoneyText cents={data.yearIncomeTotalCents} fw={600} size="sm" />
          </Stack>
          <Stack gap={0} align="center">
            <Text size="xs" c="dimmed">{t('year.expenses')}</Text>
            <MoneyText cents={data.yearExpenseTotalCents} fw={600} size="sm" />
          </Stack>
          <Stack gap={0} align="center">
            <Text size="xs" c="dimmed">{t('pots.balance')}</Text>
            <MoneyText cents={totalPotBalance} fw={600} size="sm" />
          </Stack>
        </SimpleGrid>

        {chartData.some((d) => d[t('year.income')] || d[t('year.expenses')]) && (
          <Paper shadow="xs" p="sm" withBorder>
            <CompositeChart
              h={160}
              data={chartData}
              dataKey="month"
              withLegend={false}
              valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
              series={[
                { name: t('year.income'), color: 'teal.6', type: 'bar' },
                { name: t('year.expenses'), color: 'red.6', type: 'bar' },
                { name: t('year.surplus'), color: 'blue.6', type: 'line' },
              ]}
            />
          </Paper>
        )}

        {savingsPotBalances.length > 0 && (
          <ScrollArea type="auto" offsetScrollbars>
            <Group wrap="nowrap" gap="sm">
              {savingsPotBalances.map((p) => (
                <Paper key={p.potId} component={Link} to={`/pots/${p.potId}`} shadow="xs" p="sm" withBorder miw={140} style={{ textDecoration: 'none', color: 'inherit' }}>
                  <Text size="xs" c="dimmed" truncate>{p.name}</Text>
                  <MoneyText cents={p.balanceCents} fw={700} />
                </Paper>
              ))}
            </Group>
          </ScrollArea>
        )}

        <MobileList>
          {data.months.map((m) => (
            <MobileListRow
              key={m.month}
              title={months[m.month - 1]}
              subtitle={m.status ? t(`common.${m.status}`) : t('year.createMonth')}
              trailing={m.periodId ? <MoneyText cents={m.surplusCents} colored fw={600} /> : undefined}
              to={m.periodId ? `/months/${y}/${m.month}` : undefined}
              onClick={!m.periodId && !isLocked ? () => createPeriod.mutate({ year: y, month: m.month }) : undefined}
              chevron
            />
          ))}
        </MobileList>

        {isAdmin && (
          <Group justify="center">
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
        )}

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
      </Stack>
    )
  }

  return (
    <>
      <Group justify="space-between" mb="md">
        <Group gap="sm">
          <Title order={2}>{t('year.title', { year: y })}</Title>
          {isLocked && <Badge color="red">{t('year.locked')}</Badge>}
        </Group>
        <Group gap="xs">
          <Button size="xs" variant="subtle" leftSection={<IconChartLine size={16} />} onClick={() => navigate(`/years/${y}/trends`)}>
            {t('year.trendsLink')}
          </Button>
          {isAdmin && (
            isLocked ? (
              <Button size="xs" color="orange" variant="subtle" onClick={() => setUnlockModalOpen(true)}>
                {t('year.unlockAction')}
              </Button>
            ) : (
              <Button size="xs" color="red" variant="subtle" onClick={() => setLockModalOpen(true)}>
                {t('year.lockAction')}
              </Button>
            )
          )}
        </Group>
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

      {savingsPotBalances.length > 0 && (
        <SimpleGrid cols={{ base: 2, sm: 3, md: 4 }} mb="lg">
          {savingsPotBalances.map((p) => (
            <Paper key={p.potId} component={Link} to={`/pots/${p.potId}`} shadow="xs" p="sm" withBorder style={{ textDecoration: 'none', color: 'inherit' }}>
              <Group justify="space-between" wrap="nowrap">
                <Text size="sm" fw={600}>{p.name}</Text>
                <MoneyText cents={p.balanceCents} fw={700} />
              </Group>
            </Paper>
          ))}
        </SimpleGrid>
      )}

      <Table.ScrollContainer minWidth={500}>
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
      </Table.ScrollContainer>

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
