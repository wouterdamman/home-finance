import { Link } from 'react-router-dom'
import { Title, SimpleGrid, Card, Text, Skeleton, Alert } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { usePotBalances } from '../api/hooks/usePots'
import MoneyText from '../components/MoneyText'

export default function Pots() {
  const { t } = useTranslation()
  const { data, isLoading, error } = usePotBalances()
  if (isLoading) return <Skeleton h={400} />
  if (error) return <Alert color="red">{t('common.error')}</Alert>
  return (
    <>
      <Title order={2} mb="md">{t('pots.title')}</Title>
      <SimpleGrid cols={{ base: 2, sm: 3, md: 4 }}>
        {(data ?? []).map((b) => (
          <Card key={b.potId} shadow="xs" component={Link} to={`/pots/${b.potId}`} style={{ textDecoration: 'none' }}>
            <Text fw={600} truncate>{b.name}</Text>
            <Text size="xs" c="dimmed" mb="xs">{b.kind}</Text>
            <MoneyText cents={b.balanceCents} size="lg" fw={700} colored />
          </Card>
        ))}
      </SimpleGrid>
    </>
  )
}
