import { useMemo, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Title, Paper, Skeleton, Alert, Group, ActionIcon, MultiSelect, Text } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { LineChart } from '@mantine/charts'
import { IconChevronLeft } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useYearCategoryTotals } from '../api/hooks/usePeriods'
import { formatCents } from '../lib/money'

const MONTHS_NL = ['Jan','Feb','Mar','Apr','Mei','Jun','Jul','Aug','Sep','Okt','Nov','Dec']
const MONTHS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

const CHART_COLORS = ['teal.6', 'blue.6', 'orange.6', 'grape.6', 'red.6', 'indigo.6', 'lime.7', 'pink.6']

const DEFAULT_SELECTION_COUNT = 5

export default function CategoryTrends() {
  const { year } = useParams<{ year: string }>()
  const y = Number(year)
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  const { data, isLoading, error } = useYearCategoryTotals(y)
  const months = i18n.language.startsWith('nl') ? MONTHS_NL : MONTHS_EN
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'

  const defaultSelected = useMemo(
    () => (data?.categories ?? []).slice(0, DEFAULT_SELECTION_COUNT).map((c) => String(c.id)),
    [data],
  )
  const [selected, setSelected] = useState<string[] | null>(null)
  const selectedIds = selected ?? defaultSelected

  if (isLoading) return <Skeleton h={400} />
  if (error) return <Alert color="red">{t('common.error')}</Alert>
  if (!data) return null

  const categoriesById = new Map(data.categories.map((c) => [String(c.id), c]))
  const selectedCategories = selectedIds
    .map((id) => categoriesById.get(id))
    .filter((c): c is NonNullable<typeof c> => c != null)

  const chartData = data.months.map((m) => {
    const row: Record<string, string | number> = { month: months[m.month - 1] }
    for (const cat of selectedCategories) {
      row[cat.name] = (m.values[String(cat.id)] ?? 0) / 100
    }
    return row
  })

  const series = selectedCategories.map((cat, i) => ({
    name: cat.name,
    color: CHART_COLORS[i % CHART_COLORS.length],
  }))

  return (
    <>
      <Group gap="sm" mb="md" wrap="nowrap">
        <ActionIcon variant="subtle" aria-label={t('trends.back')} onClick={() => navigate(`/years/${y}`)}>
          <IconChevronLeft size={18} />
        </ActionIcon>
        <Title order={isMobile ? 3 : 2}>{t('trends.title', { year: y })}</Title>
      </Group>

      {data.categories.length === 0 ? (
        <Text c="dimmed">{t('trends.noData')}</Text>
      ) : (
        <>
          <MultiSelect
            mb="md"
            label={t('trends.selectCategories')}
            data={data.categories.map((c) => ({ value: String(c.id), label: c.name }))}
            value={selectedIds}
            onChange={setSelected}
            searchable
            clearable
          />

          <Paper shadow="xs" p={isMobile ? 'sm' : 'md'} withBorder>
            <LineChart
              h={isMobile ? 220 : 320}
              data={chartData}
              dataKey="month"
              withLegend={!isMobile}
              valueFormatter={(v) => formatCents(Math.round(v * 100), locale)}
              series={series}
            />
          </Paper>
        </>
      )}
    </>
  )
}
