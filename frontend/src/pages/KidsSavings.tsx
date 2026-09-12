import { Link } from 'react-router-dom'
import { Title, SimpleGrid, Paper, Group, Text, Skeleton, Alert, Stack, Badge } from '@mantine/core'
import { BarChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { IconUsers } from '@tabler/icons-react'
import { useKidBalances } from '../api/hooks/useSettings'
import MoneyText from '../components/MoneyText'
import EmptyState from '../components/EmptyState'
import { useChartPalette } from '../contexts/ChartPaletteContext'

export default function KidsSavings() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const { palette } = useChartPalette()
  const { data, isLoading, error } = useKidBalances()
  const kids = data ?? []
  const chartData = kids.map((k) => ({
    name: k.name,
    [t('kids.ours')]: k.oursCents / 100,
    [t('kids.theirs')]: k.theirsCents / 100,
  }))

  if (isLoading) return <Skeleton h={200} mt="md" />
  if (error) return <Alert color="red">{t('common.error')}</Alert>

  return (
    <>
      <Title order={2} mb="md">{t('kids.title')}</Title>
      {kids.length === 0 && (
        <EmptyState message={t('kids.noEntries')} icon={<IconUsers size={22} />} />
      )}
      {kids.length > 0 && (
        <Paper shadow="xs" p="md" withBorder mb="md">
          <BarChart
            h={200}
            data={chartData}
            dataKey="name"
            type="stacked"
            withLegend
            series={[
              { name: t('kids.ours'), color: palette.categorical[0] },
              { name: t('kids.theirs'), color: palette.categorical[1] },
            ]}
            valueFormatter={(v) => new Intl.NumberFormat(locale, { style: 'currency', currency: 'EUR' }).format(v)}
          />
        </Paper>
      )}
      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }}>
        {kids.map((k) => {
          const hasReported = k.reportedBalanceCents != null
          const diff = hasReported ? k.reportedBalanceCents! - k.totalCents : 0
          return (
            <Paper key={k.kidId} component={Link} to={`/kids/${k.kidId}`} shadow="xs" p="md" withBorder style={{ textDecoration: 'none', color: 'inherit' }}>
              <Stack gap="xs">
                <Group justify="space-between" wrap="nowrap">
                  <Text fw={600}>{k.name}</Text>
                  <MoneyText cents={k.totalCents} size="lg" fw={700} />
                </Group>
                <Group justify="space-between" wrap="nowrap">
                  <Badge size="sm" variant="light" color="blue">{t('kids.ours')}</Badge>
                  <MoneyText cents={k.oursCents} size="sm" />
                </Group>
                <Group justify="space-between" wrap="nowrap">
                  <Badge size="sm" variant="light" color="green">{t('kids.theirs')}</Badge>
                  <MoneyText cents={k.theirsCents} size="sm" />
                </Group>
                {hasReported && (
                  <Group justify="space-between" wrap="nowrap" mt="xs">
                    <Text size="xs" c="dimmed">
                      {t('kids.reportedAsOf', { date: k.reportedBalanceDate ? dayjs(k.reportedBalanceDate).format('DD-MM-YYYY') : '' })}
                    </Text>
                    {diff !== 0 ? (
                      <Text size="xs" c="orange" fw={600}>{t('kids.diff')} <MoneyText cents={diff} size="xs" span /></Text>
                    ) : (
                      <Text size="xs" c="green">{t('kids.matches')}</Text>
                    )}
                  </Group>
                )}
              </Stack>
            </Paper>
          )
        })}
      </SimpleGrid>
    </>
  )
}
