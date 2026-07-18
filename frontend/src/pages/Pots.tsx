import { Link } from 'react-router-dom'
import { Title, SimpleGrid, Paper, Group, Text, Skeleton, Alert, Progress, Stack, ActionIcon, Tooltip } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { IconPigMoney, IconChartLine } from '@tabler/icons-react'
import { usePotBalances } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'

export default function Pots() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, error } = usePotBalances()
  // Carryover pots always net to zero (surplus passes through to next
  // month's income) — they aren't a savings goal, so they don't belong here.
  const savingsPots = (data ?? []).filter((p) => p.kind !== 'carryover')
  const totalSaved = savingsPots.reduce((sum, p) => sum + p.balanceCents, 0)

  if (isLoading) return <Skeleton h={200} mt="md" />
  if (error) return <Alert color="red">{t('common.error')}</Alert>

  return (
    <>
      <Group justify="space-between" mb="md">
        <Title order={2}>{t('pots.title')}</Title>
        {isMobile && (
          <Tooltip label={t('nav.trends')}>
            <ActionIcon component={Link} to="/trends" variant="subtle" aria-label={t('nav.trends')}>
              <IconChartLine size={20} />
            </ActionIcon>
          </Tooltip>
        )}
      </Group>
      {isMobile && savingsPots.length > 0 && (
        <Group justify="space-between" mb="md" px="xs">
          <Text size="sm" c="dimmed">{t('pots.totalSavedLabel', { count: savingsPots.length })}</Text>
          <MoneyText cents={totalSaved} fw={700} />
        </Group>
      )}
      {savingsPots.length === 0 && (
        <EmptyState message={t('pots.noEntries')} icon={<IconPigMoney size={22} />} />
      )}
      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }}>
        {savingsPots.map((p) => {
          const hasTarget = p.targetCents != null && p.targetCents > 0
          const progress = hasTarget ? Math.min(100, Math.max(0, (p.balanceCents / p.targetCents!) * 100)) : null
          return (
            <Paper key={p.potId} component={Link} to={`/pots/${p.potId}`} shadow="xs" p="md" withBorder style={{ textDecoration: 'none', color: 'inherit' }}>
              <Stack gap="xs">
                <Group justify="space-between" wrap="nowrap">
                  <Text fw={600}>{p.name}</Text>
                  <MoneyText cents={p.balanceCents} size="lg" fw={700} />
                </Group>
                {hasTarget && (
                  <>
                    <Progress value={progress!} color={progress! >= 100 ? 'green' : 'blue'} />
                    <Group justify="space-between" wrap="nowrap">
                      <Text size="xs" c="dimmed">
                        {t('pots.targetProgress', { percent: Math.round(progress!) })}
                        {' · '}
                        <MoneyText cents={p.targetCents!} size="xs" span />
                      </Text>
                      {p.targetDate && <Text size="xs" c="dimmed">{dayjs(p.targetDate).format('DD-MM-YYYY')}</Text>}
                    </Group>
                  </>
                )}
              </Stack>
            </Paper>
          )
        })}
      </SimpleGrid>
    </>
  )
}
