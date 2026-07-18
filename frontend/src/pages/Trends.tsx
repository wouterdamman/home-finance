import { Title, Paper, Skeleton, Alert, Text, Stack, SimpleGrid } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { CompositeChart, LineChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import { useTrendsYears, useTrendsCategoryTotals } from '../api/hooks/usePeriods'
import { formatCents } from '../lib/money'
import MoneyText from '../components/MoneyText'

const MONTHS_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

// Small multiples, not one overlaid chart: each category card carries its own
// identity via its title, so every card reuses the single brand hue rather
// than cycling a categorical palette past its safe 8-color ceiling.
const CATEGORY_LINE_COLOR = 'teal.6'
const TOP_CATEGORY_COUNT = 8

export default function Trends() {
  const { t, i18n } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const yearsQuery = useTrendsYears()
  const categoryQuery = useTrendsCategoryTotals()
  const months = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  if (yearsQuery.isLoading || categoryQuery.isLoading) return <Skeleton h={400} />
  if (yearsQuery.error || categoryQuery.error) return <Alert color="red">{t('common.error')}</Alert>
  if (!yearsQuery.data || !categoryQuery.data) return null

  const yearData = yearsQuery.data.filter((y) => y.incomeTotalCents !== 0 || y.expenseTotalCents !== 0)
  const catData = categoryQuery.data

  const totalIncome = yearData.reduce((sum, y) => sum + y.incomeTotalCents, 0)
  const totalExpenses = yearData.reduce((sum, y) => sum + y.expenseTotalCents, 0)

  const yearChartData = yearData.map((y) => ({
    year: String(y.year),
    [t('year.income')]: y.incomeTotalCents / 100,
    [t('year.expenses')]: y.expenseTotalCents / 100,
    [t('year.surplus')]: y.surplusCents / 100,
  }))

  const periodLabels = catData.entries.map((e) => `${months[e.month - 1]} '${String(e.year).slice(-2)}`)
  const topCategories = catData.categories.slice(0, TOP_CATEGORY_COUNT)
  const tickInterval = periodLabels.length > 8 ? Math.ceil(periodLabels.length / 6) : 0

  return (
    <Stack gap="xl">
      <Title order={isMobile ? 3 : 2}>{t('trends.title')}</Title>

      {yearData.length === 0 ? (
        <Text c="dimmed">{t('trends.noData')}</Text>
      ) : (
        <>
          <SimpleGrid cols={{ base: 2, sm: 4 }}>
            <Paper shadow="xs" p="md" withBorder>
              <Text size="xs" c="dimmed">{t('trends.totalIncome')}</Text>
              <MoneyText cents={totalIncome} size="lg" fw={700} />
            </Paper>
            <Paper shadow="xs" p="md" withBorder>
              <Text size="xs" c="dimmed">{t('trends.totalExpenses')}</Text>
              <MoneyText cents={totalExpenses} size="lg" fw={700} />
            </Paper>
            <Paper shadow="xs" p="md" withBorder>
              <Text size="xs" c="dimmed">{t('trends.totalSurplus')}</Text>
              <MoneyText cents={totalIncome - totalExpenses} size="lg" fw={700} colored />
            </Paper>
            <Paper shadow="xs" p="md" withBorder>
              <Text size="xs" c="dimmed">{t('trends.yearsTracked')}</Text>
              <Text size="lg" fw={700}>{yearData.length}</Text>
            </Paper>
          </SimpleGrid>

          <Stack gap="sm">
            <Title order={4}>{t('trends.yearsTitle')}</Title>
            <Paper shadow="xs" p={isMobile ? 'sm' : 'md'} withBorder>
              <CompositeChart
                h={isMobile ? 220 : 300}
                data={yearChartData}
                dataKey="year"
                withLegend={!isMobile}
                valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
                series={[
                  { name: t('year.income'), color: 'teal.6', type: 'bar' },
                  { name: t('year.expenses'), color: 'red.6', type: 'bar' },
                  { name: t('year.surplus'), color: 'blue.6', type: 'line' },
                ]}
              />
            </Paper>
          </Stack>
        </>
      )}

      <Stack gap="sm">
        <Title order={4}>{t('trends.categoriesTitle')}</Title>
        {topCategories.length === 0 ? (
          <Text c="dimmed">{t('trends.noData')}</Text>
        ) : (
          <SimpleGrid cols={{ base: 1, sm: 2, lg: topCategories.length > 6 ? 4 : 3 }}>
            {topCategories.map((cat) => {
              const data = catData.entries.map((e, i) => ({
                period: periodLabels[i],
                [cat.name]: (e.values[String(cat.id)] ?? 0) / 100,
              }))
              return (
                <Paper key={cat.id} shadow="xs" p="sm" withBorder>
                  <Text size="sm" fw={600} mb={4} truncate>{cat.name}</Text>
                  <LineChart
                    h={140}
                    data={data}
                    dataKey="period"
                    withLegend={false}
                    withDots={periodLabels.length <= 24}
                    valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
                    xAxisProps={tickInterval > 0 ? { interval: tickInterval } : undefined}
                    series={[{ name: cat.name, color: CATEGORY_LINE_COLOR }]}
                  />
                </Paper>
              )
            })}
          </SimpleGrid>
        )}
      </Stack>
    </Stack>
  )
}
