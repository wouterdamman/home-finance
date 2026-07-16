import { Link } from 'react-router-dom'
import { Title, SimpleGrid, Paper, Group, Text, Skeleton, Alert, Progress, Stack } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { IconPigMoney } from '@tabler/icons-react'
import { usePotBalances } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'

export default function Pots() {
  const { t } = useTranslation()
  const { data, isLoading, error } = usePotBalances()
  // Carryover pots always net to zero (surplus passes through to next
  // month's income) — they aren't a savings goal, so they don't belong here.
  const savingsPots = (data ?? []).filter((p) => p.kind !== 'carryover')

  if (isLoading) return <Skeleton h={200} mt="md" />
  if (error) return <Alert color="red">{t('common.error')}</Alert>

  return (
    <>
      <Title order={2} mb="md">{t('pots.title')}</Title>
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
