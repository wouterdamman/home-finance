import { useEffect, useState } from 'react'
import { Modal, Select, MultiSelect, SegmentedControl, Stack, Button, Text, Divider, NumberInput } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { WidgetConfig, ChartKind, WidgetWidth, WidgetHeight } from '../../lib/trendsDashboard'
import { MAX_INCOME_SOURCE_SLOTS, MAX_POT_SLOTS } from '../../lib/trendsDashboard'
import { MAX_COMPARE_YEARS } from '../../lib/trendsFilter'
import type { CategoryTotalsCategory, TrendsIncomeSourcesSource, TrendsPotBalancesPot } from '../../api/types'
import { useIncomeSources } from '../../api/hooks/useSettings'
import SizeGridPicker from './SizeGridPicker'
import { incomeSourceLabel } from '../../lib/incomeSourceLabel'

type BaseType = WidgetConfig['type']

const MAX_CATEGORIES = 4

// Configs without a chartKind field at all (kpi, incomeMix, itemizedIncome,
// potFlow — always bars, potTargets — not a chart, savingsRate — always a
// line). Narrows the union so `initial.chartKind` below type-checks.
function hasChartKind(config: WidgetConfig): config is WidgetConfig & { chartKind: ChartKind } {
  return config.type !== 'kpi' && config.type !== 'incomeMix' && config.type !== 'itemizedIncome'
    && config.type !== 'potFlow' && config.type !== 'potTargets' && config.type !== 'savingsRate'
    && config.type !== 'categoryShare' && config.type !== 'topDescriptions' && config.type !== 'sankeyFlow'
}

// Default card size offered when a user picks one of these types while
// *adding* a new widget (not when editing an existing one, which keeps its
// saved size) — matches the product spec for each type's typical content.
const DEFAULT_SIZE: Partial<Record<BaseType, [WidgetWidth, WidgetHeight]>> = {
  incomeSources: [2, 3],
  // 2x3 rather than 1x2: the donut plus a wrapped legend for up to 8 sources
  // does not fit two row units, and a 1-column card wraps the legend to three
  // lines on desktop.
  incomeMix: [2, 3],
  // 3 rows: a ranked list of up to 10 descriptions needs the vertical room,
  // otherwise the category labels of adjacent bars overlap.
  itemizedIncome: [2, 3],
  potBalances: [2, 3],
  potFlow: [2, 3],
  // 1x3: a list of Progress rows needs vertical room, not horizontal — it's
  // text/bars, not a chart that benefits from extra width.
  potTargets: [1, 3],
  savingsRate: [2, 3],
  categoryShare: [2, 3],
  topDescriptions: [2, 3],
  // 4x4: a three-column labeled Sankey needs real room on both axes —
  // narrower/shorter and the node labels start colliding.
  sankeyFlow: [4, 4],
}

const MONTH_OPTIONS_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTH_OPTIONS_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

interface Props {
  opened: boolean
  onClose: () => void
  onSubmit: (config: WidgetConfig, width: WidgetWidth, height: WidgetHeight) => void
  categories: CategoryTotalsCategory[]
  incomeSources: TrendsIncomeSourcesSource[]
  pots: TrendsPotBalancesPot[]
  years: number[]
  initial?: WidgetConfig
  initialWidth?: WidgetWidth
  initialHeight?: WidgetHeight
}

export default function WidgetModal({ opened, onClose, onSubmit, categories, incomeSources, pots, years, initial, initialWidth, initialHeight }: Props) {
  const { t, i18n } = useTranslation()
  const monthNames = i18n.language.startsWith('nl') ? MONTH_OPTIONS_NL : MONTH_OPTIONS_EN
  const firstYear = years[0] ?? new Date().getFullYear()
  const lastYear = years[years.length - 1] ?? new Date().getFullYear()
  // itemizedIncome's source select only makes sense for itemized sources —
  // reusing the existing Settings hook rather than adding a new one.
  const { data: allIncomeSourceDefs } = useIncomeSources()
  const itemizedSourceOptions = (allIncomeSourceDefs ?? []).filter((s) => s.isItemized && !s.archivedAt)

  const [type, setType] = useState<BaseType>(initial?.type ?? 'categoryChart')
  const [metric, setMetric] = useState<string>(initial?.type === 'kpi' ? initial.metric : 'income')
  const [categoryIds, setCategoryIds] = useState<string[]>(
    initial?.type === 'categoryChart' ? initial.categoryIds.map(String) : (categories[0] ? [String(categories[0].id)] : []),
  )
  const [chartKind, setChartKind] = useState<ChartKind>(
    initial && hasChartKind(initial) ? initial.chartKind : 'line',
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
  const [incomeSourceIds, setIncomeSourceIds] = useState<string[]>(
    initial?.type === 'incomeSources' ? initial.sourceIds.map(String) : (incomeSources[0] ? [String(incomeSources[0].id)] : []),
  )
  const [itemizedSourceId, setItemizedSourceId] = useState<string | null>(
    initial?.type === 'itemizedIncome' ? String(initial.sourceId) : (itemizedSourceOptions[0] ? String(itemizedSourceOptions[0].id) : null),
  )
  const [potIds, setPotIds] = useState<string[]>(
    initial?.type === 'potBalances' || initial?.type === 'potFlow' ? initial.potIds.map(String) : (pots[0] ? [String(pots[0].id)] : []),
  )
  const [savingsRateYear, setSavingsRateYear] = useState<string | null>(
    initial?.type === 'savingsRate' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null),
  )
  // 'all' sentinel rather than '' — Mantine Select needs a non-empty value
  // for every option, and categoryId: null ("all categories") has to round-trip.
  const ALL_CATEGORIES_VALUE = 'all'
  const [topDescriptionsCategory, setTopDescriptionsCategory] = useState<string>(
    initial?.type === 'topDescriptions' ? (initial.categoryId == null ? ALL_CATEGORIES_VALUE : String(initial.categoryId)) : ALL_CATEGORIES_VALUE,
  )
  const [topDescriptionsLimit, setTopDescriptionsLimit] = useState<number>(
    initial?.type === 'topDescriptions' ? initial.limit : 10,
  )
  const [sankeyYear, setSankeyYear] = useState<string | null>(
    initial?.type === 'sankeyFlow' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null),
  )
  const [width, setWidth] = useState<WidgetWidth>(initialWidth ?? 1)
  const [height, setHeight] = useState<WidgetHeight>(initialHeight ?? 2)

  useEffect(() => {
    if (!opened) return
    setType(initial?.type ?? 'categoryChart')
    setMetric(initial?.type === 'kpi' ? initial.metric : 'income')
    setCategoryIds(initial?.type === 'categoryChart' ? initial.categoryIds.map(String) : (categories[0] ? [String(categories[0].id)] : []))
    setChartKind(initial && hasChartKind(initial) ? initial.chartKind : 'line')
    setMonthYear(initial?.type === 'monthCompare' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null))
    setSelectedMonths(initial?.type === 'monthCompare' ? initial.months.map(String) : ['1', '2'])
    setFromYear(initial?.type === 'allTimeTrend' ? String(initial.fromYear) : String(firstYear))
    setToYear(initial?.type === 'allTimeTrend' ? String(initial.toYear) : String(lastYear))
    setSingleMonth(initial?.type === 'monthAcrossYears' ? String(initial.month) : '1')
    setSelectedYears(initial?.type === 'monthAcrossYears' ? initial.years.map(String) : years.slice(-2).map(String))
    setIncomeSourceIds(initial?.type === 'incomeSources' ? initial.sourceIds.map(String) : (incomeSources[0] ? [String(incomeSources[0].id)] : []))
    setItemizedSourceId(initial?.type === 'itemizedIncome' ? String(initial.sourceId) : (itemizedSourceOptions[0] ? String(itemizedSourceOptions[0].id) : null))
    setPotIds(initial?.type === 'potBalances' || initial?.type === 'potFlow' ? initial.potIds.map(String) : (pots[0] ? [String(pots[0].id)] : []))
    setSavingsRateYear(initial?.type === 'savingsRate' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null))
    setTopDescriptionsCategory(initial?.type === 'topDescriptions' ? (initial.categoryId == null ? ALL_CATEGORIES_VALUE : String(initial.categoryId)) : ALL_CATEGORIES_VALUE)
    setTopDescriptionsLimit(initial?.type === 'topDescriptions' ? initial.limit : 10)
    setSankeyYear(initial?.type === 'sankeyFlow' ? String(initial.year) : (years[years.length - 1] ? String(years[years.length - 1]) : null))
    setWidth(initialWidth ?? 1)
    setHeight(initialHeight ?? 2)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened, initial, initialWidth, initialHeight])

  // The itemized-source list arrives asynchronously, so its default has to
  // be filled in once it loads. This deliberately does NOT live in the
  // reset effect above: having that effect depend on the list's length made
  // it re-run when the query resolved while the modal was already open,
  // wiping every field the user had configured in the meantime.
  useEffect(() => {
    if (!opened) return
    setItemizedSourceId((current) => current ?? (itemizedSourceOptions[0] ? String(itemizedSourceOptions[0].id) : null))
  }, [opened, itemizedSourceOptions])

  const canSubmit = (type !== 'categoryChart' || categoryIds.length >= 1)
    && (type !== 'monthCompare' || (monthYear != null && selectedMonths.length >= 1))
    && (type !== 'allTimeTrend' || (fromYear != null && toYear != null && Number(fromYear) <= Number(toYear)))
    && (type !== 'monthAcrossYears' || (singleMonth != null && selectedYears.length >= 1))
    && (type !== 'incomeSources' || incomeSourceIds.length >= 1)
    && (type !== 'itemizedIncome' || itemizedSourceId != null)
    && (type !== 'potBalances' || potIds.length >= 1)
    && (type !== 'potFlow' || potIds.length >= 1)
    && (type !== 'savingsRate' || savingsRateYear != null)
    && (type !== 'sankeyFlow' || sankeyYear != null)

  const handleSubmit = () => {
    if (type === 'kpi') {
      onSubmit({ type: 'kpi', metric: metric as 'income' | 'expenses' | 'surplus' | 'yearsTracked' }, width, height)
    } else if (type === 'categoryChart') {
      if (categoryIds.length === 0) return
      onSubmit({ type: 'categoryChart', categoryIds: categoryIds.map(Number), chartKind }, width, height)
    } else if (type === 'monthCompare') {
      if (!monthYear || selectedMonths.length === 0) return
      onSubmit({ type: 'monthCompare', year: Number(monthYear), months: selectedMonths.map(Number), chartKind }, width, height)
    } else if (type === 'allTimeTrend') {
      if (!fromYear || !toYear) return
      onSubmit({ type: 'allTimeTrend', fromYear: Number(fromYear), toYear: Number(toYear), chartKind }, width, height)
    } else if (type === 'monthAcrossYears') {
      if (!singleMonth || selectedYears.length === 0) return
      onSubmit({ type: 'monthAcrossYears', month: Number(singleMonth), years: selectedYears.map(Number), chartKind }, width, height)
    } else if (type === 'incomeSources') {
      if (incomeSourceIds.length === 0) return
      onSubmit({ type: 'incomeSources', sourceIds: incomeSourceIds.map(Number), chartKind }, width, height)
    } else if (type === 'incomeMix') {
      onSubmit({ type: 'incomeMix' }, width, height)
    } else if (type === 'itemizedIncome') {
      if (itemizedSourceId == null) return
      onSubmit({ type: 'itemizedIncome', sourceId: Number(itemizedSourceId) }, width, height)
    } else if (type === 'potBalances') {
      if (potIds.length === 0) return
      onSubmit({ type: 'potBalances', potIds: potIds.map(Number), chartKind }, width, height)
    } else if (type === 'potFlow') {
      if (potIds.length === 0) return
      onSubmit({ type: 'potFlow', potIds: potIds.map(Number) }, width, height)
    } else if (type === 'potTargets') {
      onSubmit({ type: 'potTargets' }, width, height)
    } else if (type === 'savingsRate') {
      if (savingsRateYear == null) return
      onSubmit({ type: 'savingsRate', year: Number(savingsRateYear) }, width, height)
    } else if (type === 'categoryShare') {
      onSubmit({ type: 'categoryShare' }, width, height)
    } else if (type === 'topDescriptions') {
      const categoryId = topDescriptionsCategory === ALL_CATEGORIES_VALUE ? null : Number(topDescriptionsCategory)
      onSubmit({ type: 'topDescriptions', categoryId, limit: topDescriptionsLimit }, width, height)
    } else {
      if (sankeyYear == null) return
      onSubmit({ type: 'sankeyFlow', year: Number(sankeyYear) }, width, height)
    }
    onClose()
  }

  const handleTypeChange = (next: BaseType) => {
    setType(next)
    if (!initial) {
      const size = DEFAULT_SIZE[next]
      if (size) {
        setWidth(size[0])
        setHeight(size[1])
      }
    }
  }

  return (
    <Modal opened={opened} onClose={onClose} title={initial ? t('trends.configureWidget') : t('trends.addWidget')}>
      <Stack gap="sm">
        {!initial && (
          <Select
            label={t('trends.widgetTypeLabel')}
            value={type}
            onChange={(v) => v && handleTypeChange(v as BaseType)}
            allowDeselect={false}
            data={[
              { value: 'kpi', label: t('trends.widgetType_kpi') },
              { value: 'categoryChart', label: t('trends.widgetType_categoryChart') },
              { value: 'monthCompare', label: t('trends.widgetType_monthCompare') },
              { value: 'allTimeTrend', label: t('trends.widgetType_allTimeTrend') },
              { value: 'monthAcrossYears', label: t('trends.widgetType_monthAcrossYears') },
              { value: 'incomeSources', label: t('trends.widgetType_incomeSources') },
              { value: 'incomeMix', label: t('trends.widgetType_incomeMix') },
              { value: 'itemizedIncome', label: t('trends.widgetType_itemizedIncome') },
              { value: 'potBalances', label: t('trends.widgetType_potBalances') },
              { value: 'potFlow', label: t('trends.widgetType_potFlow') },
              { value: 'potTargets', label: t('trends.widgetType_potTargets') },
              { value: 'savingsRate', label: t('trends.widgetType_savingsRate') },
              { value: 'categoryShare', label: t('trends.widgetType_categoryShare') },
              { value: 'topDescriptions', label: t('trends.widgetType_topDescriptions') },
              { value: 'sankeyFlow', label: t('trends.widgetType_sankeyFlow') },
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
            <MultiSelect
              label={t('settings.categories')}
              data={categories.map((c) => ({ value: String(c.id), label: c.name }))}
              value={categoryIds}
              onChange={setCategoryIds}
              maxValues={MAX_CATEGORIES}
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

        {type === 'incomeSources' && (
          <>
            <MultiSelect
              label={t('trends.incomeSourcesLabel')}
              data={incomeSources.map((s) => ({ value: String(s.id), label: incomeSourceLabel(s, t) }))}
              value={incomeSourceIds}
              onChange={setIncomeSourceIds}
              maxValues={MAX_INCOME_SOURCE_SLOTS}
              searchable
            />
            {incomeSources.length === 0 && <Text size="sm" c="dimmed">{t('trends.noData')}</Text>}
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

        {type === 'incomeMix' && (
          <Text size="sm" c="dimmed">{t('trends.incomeMixHint')}</Text>
        )}

        {type === 'itemizedIncome' && (
          <>
            <Select
              label={t('trends.itemizedIncomeSourceLabel')}
              data={itemizedSourceOptions.map((s) => ({ value: String(s.id), label: s.name }))}
              value={itemizedSourceId}
              onChange={setItemizedSourceId}
              allowDeselect={false}
            />
            {itemizedSourceOptions.length === 0 && <Text size="sm" c="dimmed">{t('trends.noItemizedIncomeSources')}</Text>}
          </>
        )}

        {type === 'potBalances' && (
          <>
            <MultiSelect
              label={t('trends.potsLabel')}
              data={pots.map((p) => ({ value: String(p.id), label: p.name }))}
              value={potIds}
              onChange={setPotIds}
              maxValues={MAX_POT_SLOTS}
              searchable
            />
            {pots.length === 0 && <Text size="sm" c="dimmed">{t('trends.noData')}</Text>}
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

        {type === 'potFlow' && (
          <>
            <MultiSelect
              label={t('trends.potsLabel')}
              data={pots.map((p) => ({ value: String(p.id), label: p.name }))}
              value={potIds}
              onChange={setPotIds}
              maxValues={MAX_POT_SLOTS}
              searchable
            />
            {pots.length === 0 && <Text size="sm" c="dimmed">{t('trends.noData')}</Text>}
          </>
        )}

        {type === 'potTargets' && (
          <Text size="sm" c="dimmed">{t('trends.potTargetsHint')}</Text>
        )}

        {type === 'savingsRate' && (
          <Select
            label={t('settings.year')}
            data={years.map(String)}
            value={savingsRateYear}
            onChange={setSavingsRateYear}
            allowDeselect={false}
          />
        )}

        {type === 'categoryShare' && (
          <Text size="sm" c="dimmed">{t('trends.categoryShareHint')}</Text>
        )}

        {type === 'topDescriptions' && (
          <>
            <Select
              label={t('trends.topDescriptionsCategoryLabel')}
              data={[
                { value: ALL_CATEGORIES_VALUE, label: t('trends.allCategories') },
                ...categories.map((c) => ({ value: String(c.id), label: c.name })),
              ]}
              value={topDescriptionsCategory}
              onChange={(v) => v && setTopDescriptionsCategory(v)}
              allowDeselect={false}
            />
            <NumberInput
              label={t('trends.topDescriptionsLimitLabel')}
              value={topDescriptionsLimit}
              onChange={(v) => setTopDescriptionsLimit(typeof v === 'number' ? v : Number(v) || 10)}
              min={3}
              max={25}
            />
          </>
        )}

        {type === 'sankeyFlow' && (
          <Select
            label={t('settings.year')}
            data={years.map(String)}
            value={sankeyYear}
            onChange={setSankeyYear}
            allowDeselect={false}
          />
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
