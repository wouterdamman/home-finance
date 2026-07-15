import { Link } from 'react-router-dom'
import { Title, SimpleGrid, Paper, Group, Text, Skeleton, Alert } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { usePotBalances } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'

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
        <Text c="dimmed">{t('pots.noEntries')}</Text>
      )}
      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }}>
        {savingsPots.map((p) => (
          <Paper key={p.potId} component={Link} to={`/pots/${p.potId}`} shadow="xs" p="md" withBorder style={{ textDecoration: 'none', color: 'inherit' }}>
            <Group justify="space-between" wrap="nowrap">
              <Text fw={600}>{p.name}</Text>
              <MoneyText cents={p.balanceCents} size="lg" fw={700} />
            </Group>
          </Paper>
        ))}
      </SimpleGrid>
    </>
  )
}
