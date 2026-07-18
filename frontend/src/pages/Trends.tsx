import { useMemo, useState } from 'react'
import { Title, Paper, Skeleton, Alert, MultiSelect, Text, Stack } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { CompositeChart, LineChart } from '@mantine/charts'
import { useTranslation } from 'react-i18next'
import { useTrendsYears, useTrendsCategoryTotals } from '../api/hooks/usePeriods'
import { formatCents } from '../lib/money'

const MONTHS_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

const CHART_COLORS = ['teal.6', 'blue.6', 'orange.6', 'grape.6', 'red.6', 'indigo.6', 'lime.7', 'pink.6']

const DEFAULT_SELECTION_COUNT = 5

export default function Trends() {
  const { t, i18n } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const yearsQuery = useTrendsYears()
  const categoryQuery = useTrendsCategoryTotals()
  const months = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  const defaultSelected = useMemo(
    () => (categoryQuery.data?.categories ?? []).slice(0, DEFAULT_SELECTION_COUNT).map((c) => String(c.id)),
    [categoryQuery.data],
  )
  const [selected, setSelected] = useState<string[] | null>(null)
  const selectedIds = selected ?? defaultSelected

  if (yearsQuery.isLoading || categoryQuery.isLoading) return <Skeleton h={400} />
  if (yearsQuery.error || categoryQuery.error) return <Alert color="red">{t('common.error')}</Alert>
  if (!yearsQuery.data || !categoryQuery.data) return null

  const yearData = yearsQuery.data
  const catData = categoryQuery.data

  const yearChartData = yearData.map((y) => ({
    year: String(y.year),
    [t('year.income')]: y.incomeTotalCents / 100,
    [t('year.expenses')]: y.expenseTotalCents / 100,
    [t('year.surplus')]: y.surplusCents / 100,
  }))

  const categoriesById = new Map(catData.categories.map((c) => [String(c.id), c]))
  const selectedCategories = selectedIds
    .map((id) => categoriesById.get(id))
    .filter((c): c is NonNullable<typeof c> => c != null)

  const categoryChartData = catData.entries.map((e) => {
    const shortYear = String(e.year).slice(-2)
    const row: Record<string, string | number> = { period: `${months[e.month - 1]} '${shortYear}` }
    for (const cat of selectedCategories) {
      row[cat.name] = (e.values[String(cat.id)] ?? 0) / 100
    }
    return row
  })

  const series = selectedCategories.map((cat, i) => ({
    name: cat.name,
    color: CHART_COLORS[i % CHART_COLORS.length],
  }))

  return (
    <Stack gap="xl">
      <Title order={isMobile ? 3 : 2}>{t('trends.title')}</Title>

      <Stack gap="sm">
        <Title order={4}>{t('trends.yearsTitle')}</Title>
        {yearChartData.length === 0 ? (
          <Text c="dimmed">{t('trends.noData')}</Text>
        ) : (
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
        )}
      </Stack>

      <Stack gap="sm">
        <Title order={4}>{t('trends.categoriesTitle')}</Title>
        {catData.categories.length === 0 ? (
          <Text c="dimmed">{t('trends.noData')}</Text>
        ) : (
          <>
            <MultiSelect
              label={t('trends.selectCategories')}
              data={catData.categories.map((c) => ({ value: String(c.id), label: c.name }))}
              value={selectedIds}
              onChange={setSelected}
              searchable
              clearable
            />
            <Paper shadow="xs" p={isMobile ? 'sm' : 'md'} withBorder>
              <LineChart
                h={isMobile ? 220 : 320}
                data={categoryChartData}
                dataKey="period"
                withLegend={!isMobile}
                valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
                series={series}
              />
            </Paper>
          </>
        )}
      </Stack>
    </Stack>
  )
}
