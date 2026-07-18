import { useEffect, useState } from 'react'
import { Modal, Select, MultiSelect, SegmentedControl, Stack, Button, Text, Divider } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { WidgetConfig, ChartKind, WidgetWidth, WidgetHeight } from '../../lib/trendsDashboard'
import { MAX_COMPARE_YEARS } from '../../lib/trendsFilter'
import type { CategoryTotalsCategory } from '../../api/types'
import SizeGridPicker from './SizeGridPicker'

type BaseType = WidgetConfig['type']

const MONTH_OPTIONS_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTH_OPTIONS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

interface Props {
  opened: boolean
  onClose: () => void
  onSubmit: (config: WidgetConfig, width: WidgetWidth, height: WidgetHeight) => void
  categories: CategoryTotalsCategory[]
  years: number[]
  initial?: WidgetConfig
  initialWidth?: WidgetWidth
  initialHeight?: WidgetHeight
}

export default function WidgetModal({ opened, onClose, onSubmit, categories, years, initial, initialWidth, initialHeight }: Props) {
  const { t, i18n } = useTranslation()
  const monthNames = i18n.language.startsWith('nl') ? MONTH_OPTIONS_NL : MONTH_OPTIONS_EN
  const firstYear = years[0] ?? new Date().getFullYear()
  const lastYear = years[years.length - 1] ?? new Date().getFullYear()

  const [type, setType] = useState<BaseType>(initial?.type ?? 'categoryChart')
  const [metric, setMetric] = useState<string>(initial?.type === 'kpi' ? initial.metric : 'income')
  const [categoryId, setCategoryId] = useState<string | null>(
    initial?.type === 'categoryChart' ? String(initial.categoryId) : (categories[0] ? String(categories[0].id) : null),
  )
  const [chartKind, setChartKind] = useState<ChartKind>(
    initial && initial.type !== 'kpi' ? initial.chartKind : 'line',
  )
  const [monthYear, setMonthYear] = useState<string | null>(
    initial?.type === 'monthCompare' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null),
  )
  const [selectedMonths, setSelectedMonths] = useState<string[]>(
    initial?.type === 'monthCompare' ? initial.months.map(String) : ['1', '2'],
  )
  const [fromYear, setFromYear] = useState<string | null>(
    initial?.type === 'allTimeTrend' ? String(initial.fromYear) : String(firstYear),
  )
  const [toYear, setToYear] = useState<string | null>(
    initial?.type === 'allTimeTrend' ? String(initial.toYear) : String(lastYear),
  )
  const [singleMonth, setSingleMonth] = useState<string | null>(
    initial?.type === 'monthAcrossYears' ? String(initial.month) : '1',
  )
  const [selectedYears, setSelectedYears] = useState<string[]>(
    initial?.type === 'monthAcrossYears' ? initial.years.map(String) : years.slice(-2).map(String),
  )
  const [width, setWidth] = useState<WidgetWidth>(initialWidth ?? 1)
  const [height, setHeight] = useState<WidgetHeight>(initialHeight ?? 2)

  useEffect(() => {
    if (!opened) return
    setType(initial?.type ?? 'categoryChart')
    setMetric(initial?.type === 'kpi' ? initial.metric : 'income')
    setCategoryId(initial?.type === 'categoryChart' ? String(initial.categoryId) : (categories[0] ? String(categories[0].id) : null))
    setChartKind(initial && initial.type !== 'kpi' ? initial.chartKind : 'line')
    setMonthYear(initial?.type === 'monthCompare' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null))
    setSelectedMonths(initial?.type === 'monthCompare' ? initial.months.map(String) : ['1', '2'])
    setFromYear(initial?.type === 'allTimeTrend' ? String(initial.fromYear) : String(firstYear))
    setToYear(initial?.type === 'allTimeTrend' ? String(initial.toYear) : String(lastYear))
    setSingleMonth(initial?.type === 'monthAcrossYears' ? String(initial.month) : '1')
    setSelectedYears(initial?.type === 'monthAcrossYears' ? initial.years.map(String) : years.slice(-2).map(String))
    setWidth(initialWidth ?? 1)
    setHeight(initialHeight ?? 2)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened, initial, initialWidth, initialHeight])

  const canSubmit = (type !== 'categoryChart' || categoryId != null)
    && (type !== 'monthCompare' || (monthYear != null && selectedMonths.length >= 1))
    && (type !== 'allTimeTrend' || (fromYear != null && toYear != null && Number(fromYear) <= Number(toYear)))
    && (type !== 'monthAcrossYears' || (singleMonth != null && selectedYears.length >= 1))

  const handleSubmit = () => {
    if (type === 'kpi') {
      onSubmit({ type: 'kpi', metric: metric as 'income' | 'expenses' | 'surplus' | 'yearsTracked' }, width, height)
    } else if (type === 'categoryChart') {
      if (!categoryId) return
      onSubmit({ type: 'categoryChart', categoryId: Number(categoryId), chartKind }, width, height)
    } else if (type === 'monthCompare') {
      if (!monthYear || selectedMonths.length === 0) return
      onSubmit({ type: 'monthCompare', year: Number(monthYear), months: selectedMonths.map(Number), chartKind }, width, height)
    } else if (type === 'allTimeTrend') {
      if (!fromYear || !toYear) return
      onSubmit({ type: 'allTimeTrend', fromYear: Number(fromYear), toYear: Number(toYear), chartKind }, width, height)
    } else if (type === 'monthAcrossYears') {
      if (!singleMonth || selectedYears.length === 0) return
      onSubmit({ type: 'monthAcrossYears', month: Number(singleMonth), years: selectedYears.map(Number), chartKind }, width, height)
    } else {
      onSubmit({ type: 'yearCompare', chartKind }, width, height)
    }
    onClose()
  }

  return (
    <Modal opened={opened} onClose={onClose} title={initial ? t('trends.configureWidget') : t('trends.addWidget')}>
      <Stack gap="sm">
        {!initial && (
          <Select
            label={t('trends.widgetTypeLabel')}
            value={type}
            onChange={(v) => v && setType(v as BaseType)}
            allowDeselect={false}
            data={[
              { value: 'kpi', label: t('trends.widgetType_kpi') },
              { value: 'categoryChart', label: t('trends.widgetType_categoryChart') },
              { value: 'yearCompare', label: t('trends.widgetType_yearCompare') },
              { value: 'monthCompare', label: t('trends.widgetType_monthCompare') },
              { value: 'allTimeTrend', label: t('trends.widgetType_allTimeTrend') },
              { value: 'monthAcrossYears', label: t('trends.widgetType_monthAcrossYears') },
            ]}
          />
        )}

        {type === 'kpi' && (
          <Select
            label={t('trends.metric')}
            data={[
              { value: 'income', label: t('trends.metric_income') },
              { value: 'expenses', label: t('trends.metric_expenses') },
              { value: 'surplus', label: t('trends.metric_surplus') },
              { value: 'yearsTracked', label: t('trends.metric_yearsTracked') },
            ]}
            value={metric}
            onChange={(v) => v && setMetric(v)}
          />
        )}

        {type === 'categoryChart' && (
          <>
            <Select
              label={t('settings.categories')}
              data={categories.map((c) => ({ value: String(c.id), label: c.name }))}
              value={categoryId}
              onChange={setCategoryId}
              searchable
            />
            {categories.length === 0 && <Text size="sm" c="dimmed">{t('trends.noData')}</Text>}
            <SegmentedControl
              fullWidth
              value={chartKind}
              onChange={(v) => setChartKind(v as ChartKind)}
              data={[
                { label: t('trends.chartKind_line'), value: 'line' },
                { label: t('trends.chartKind_bar'), value: 'bar' },
              ]}
            />
          </>
        )}

        {type === 'yearCompare' && (
          <SegmentedControl
            fullWidth
            value={chartKind}
            onChange={(v) => setChartKind(v as ChartKind)}
            data={[
              { label: t('trends.chartKind_line'), value: 'line' },
              { label: t('trends.chartKind_bar'), value: 'bar' },
            ]}
          />
        )}

        {type === 'monthCompare' && (
          <>
            <Select
              label={t('settings.year')}
              data={years.map(String)}
              value={monthYear}
              onChange={setMonthYear}
            />
            <MultiSelect
              label={t('trends.selectMonthsHint')}
              data={monthNames.map((label, i) => ({ value: String(i + 1), label }))}
              value={selectedMonths}
              onChange={setSelectedMonths}
              maxValues={MAX_COMPARE_YEARS}
            />
            <SegmentedControl
              fullWidth
              value={chartKind}
              onChange={(v) => setChartKind(v as ChartKind)}
              data={[
                { label: t('trends.chartKind_line'), value: 'line' },
                { label: t('trends.chartKind_bar'), value: 'bar' },
              ]}
            />
          </>
        )}

        {type === 'allTimeTrend' && (
          <>
            <Select
              label={t('trends.fromYear')}
              data={years.map(String)}
              value={fromYear}
              onChange={setFromYear}
              allowDeselect={false}
            />
            <Select
              label={t('trends.toYear')}
              data={years.map(String)}
              value={toYear}
              onChange={setToYear}
              allowDeselect={false}
            />
            {fromYear != null && toYear != null && Number(fromYear) > Number(toYear) && (
              <Text size="sm" c="red">{t('trends.invalidYearRange')}</Text>
            )}
            <SegmentedControl
              fullWidth
              value={chartKind}
              onChange={(v) => setChartKind(v as ChartKind)}
              data={[
                { label: t('trends.chartKind_line'), value: 'line' },
                { label: t('trends.chartKind_bar'), value: 'bar' },
              ]}
            />
          </>
        )}

        {type === 'monthAcrossYears' && (
          <>
            <Select
              label={t('trends.month')}
              data={monthNames.map((label, i) => ({ value: String(i + 1), label }))}
              value={singleMonth}
              onChange={setSingleMonth}
              allowDeselect={false}
            />
            <MultiSelect
              label={t('trends.selectYearsHint')}
              data={years.map(String)}
              value={selectedYears}
              onChange={setSelectedYears}
              maxValues={MAX_COMPARE_YEARS}
            />
            <SegmentedControl
              fullWidth
              value={chartKind}
              onChange={(v) => setChartKind(v as ChartKind)}
              data={[
                { label: t('trends.chartKind_line'), value: 'line' },
                { label: t('trends.chartKind_bar'), value: 'bar' },
              ]}
            />
          </>
        )}

        <Divider my={4} />
        <SizeGridPicker width={width} height={height} onChange={(w, h) => { setWidth(w); setHeight(h) }} />

        <Button disabled={!canSubmit} onClick={handleSubmit}>
          {initial ? t('common.save') : t('common.add')}
        </Button>
      </Stack>
    </Modal>
  )
}
