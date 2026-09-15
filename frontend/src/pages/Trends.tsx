import { useCallback, useEffect, useMemo, useState } from 'react'
import { Title, Skeleton, Alert, Stack, SimpleGrid, Group, Select, ActionIcon, Tooltip, Button, Chip } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { IconPencil, IconCheck, IconPlus, IconEye, IconArrowsLeftRight, IconTrash } from '@tabler/icons-react'
import {
  DndContext, closestCenter, PointerSensor, TouchSensor, useSensor, useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { SortableContext, rectSortingStrategy, arrayMove } from '@dnd-kit/sortable'
import { useTrendsYears, useTrendsCategoryTotals, useYears } from '../api/hooks/usePeriods'
import type { TrendsFilter } from '../lib/trendsFilter'
import { loadTrendsFilter, saveTrendsFilter } from '../lib/trendsFilter'
import type { Widget, WidgetConfig, WidgetWidth, WidgetHeight } from '../lib/trendsDashboard'
import { loadDashboard, saveDashboard, defaultWidgets, newWidget } from '../lib/trendsDashboard'
import type { CategoryTotalsCategory } from '../api/types'
import WidgetFrame from '../components/trends/WidgetFrame'
import KpiWidget from '../components/trends/KpiWidget'
import CategoryWidget from '../components/trends/CategoryWidget'
import MonthCompareWidget from '../components/trends/MonthCompareWidget'
import AllTimeTrendWidget from '../components/trends/AllTimeTrendWidget'
import MonthAcrossYearsWidget from '../components/trends/MonthAcrossYearsWidget'
import WidgetModal from '../components/trends/WidgetModal'
import EmptyState from '../components/EmptyState'

const MONTH_NAMES_NL = ['jan','feb','mrt','apr','mei','jun','jul','aug','sep','okt','nov','dec']
const MONTH_NAMES_EN = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

// A fixed row-track height (rather than auto-sized rows) is what lets
// `gridAutoFlow: dense` pack a short widget into the space beside a tall
// one — auto rows size to their tallest occupant, which just stretches
// shorter siblings with dead space instead of stacking anything.
const ROW_UNIT_PX = 90

function widgetTitle(config: WidgetConfig, categories: CategoryTotalsCategory[], t: (key: string, opts?: Record<string, unknown>) => string, monthNames: string[]): string {
  if (config.type === 'kpi') return t(`trends.metric_${config.metric}`)
  if (config.type === 'monthCompare') return t('trends.monthsTitle', { year: config.year })
  if (config.type === 'allTimeTrend') return t('trends.allTimeTrendTitle', { fromYear: config.fromYear, toYear: config.toYear })
  if (config.type === 'monthAcrossYears') return t('trends.monthAcrossYearsTitle', { month: monthNames[config.month - 1] })
  const names = config.categoryIds.map((id) => categories.find((c) => c.id === id)?.name).filter((n): n is string => n != null)
  return names.length > 0 ? names.join(', ') : t('trends.unknownCategory')
}

export default function Trends() {
  const { t, i18n } = useTranslation()
  const monthNames = i18n.language.startsWith('nl') ? MONTH_NAMES_NL : MONTH_NAMES_EN
  const isMobile = useMediaQuery('(max-width: 47.99em)')
  // Matches the SimpleGrid's own `sm`/`lg` breakpoints below (cols base:1,
  // sm:2, lg:4) so a widget's configured span never exceeds the grid's
  // actual column count — spanning more than the explicit columns forces
  // CSS Grid to add implicit ones, which can push the grid wider than its
  // container.
  const isMediumWidth = useMediaQuery('(max-width: 74.99em)')
  const maxWidgetWidth: WidgetWidth = isMobile ? 1 : isMediumWidth ? 2 : 4
  const yearsQuery = useTrendsYears()
  const categoryQuery = useTrendsCategoryTotals()
  const registeredYearsQuery = useYears()

  const [dashboard, setDashboard] = useState<Widget[] | null>(() => loadDashboard())
  const [filter, setFilter] = useState<TrendsFilter | null>(null)
  const [editMode, setEditMode] = useState(false)
  const [modalOpened, setModalOpened] = useState(false)
  const [editingWidgetId, setEditingWidgetId] = useState<string | null>(null)

  // Every year registered in the app (Settings > Years) — same list the
  // sidebar shows — so the selector never looks like it's "missing" a year
  // the user can clearly see exists elsewhere. Years with no periods yet
  // just render as zero everywhere (KpiWidget/CategoryWidget already
  // default missing data to 0).
  const registeredYears = useMemo(() => registeredYearsQuery.data ?? [], [registeredYearsQuery.data])
  const allYears = useMemo(() => yearsQuery.data ?? [], [yearsQuery.data])

  // Deliberately not gated on `registeredYears.length > 0`: a fresh install
  // with no years registered yet would otherwise leave both `dashboard` and
  // `filter` null forever, and the page stuck on its skeleton.
  useEffect(() => {
    if (dashboard === null && categoryQuery.data) {
      const lastYear = registeredYears.length > 0 ? Math.max(...registeredYears) : new Date().getFullYear()
      const topCategoryIds = categoryQuery.data.categories.map((c) => c.id)
      const recentMonths = categoryQuery.data.entries
        .filter((e) => e.year === lastYear)
        .map((e) => e.month)
        .sort((a, b) => a - b)
      const generated = defaultWidgets(registeredYears, topCategoryIds, recentMonths)
      setDashboard(generated)
      saveDashboard(generated)
    }
  }, [dashboard, registeredYears, categoryQuery.data])

  useEffect(() => {
    if (filter === null && !registeredYearsQuery.isLoading) {
      setFilter(loadTrendsFilter(registeredYears))
    }
  }, [filter, registeredYears, registeredYearsQuery.isLoading])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 150, tolerance: 5 } }),
  )

  const updateFilter = useCallback((next: TrendsFilter) => {
    setFilter(next)
    saveTrendsFilter(next)
  }, [])

  const commitDashboard = useCallback((update: (prev: Widget[]) => Widget[]) => {
    setDashboard((prev) => {
      if (prev === null) return prev
      const next = update(prev)
      saveDashboard(next)
      return next
    })
  }, [])

  const handleSetVisible = useCallback((id: string, visible: boolean) => {
    commitDashboard((prev) => prev.map((w) => (w.id === id ? { ...w, visible } : w)))
  }, [commitDashboard])

  const handleDeleteWidget = useCallback((id: string) => {
    commitDashboard((prev) => prev.filter((w) => w.id !== id))
  }, [commitDashboard])

  // Reordering only moves the *visible* widgets among themselves; hidden ones
  // keep their original slots so re-showing one later drops it back where it
  // was instead of at the end of the grid.
  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    commitDashboard((prev) => {
      const visible = prev.filter((w) => w.visible)
      const oldIndex = visible.findIndex((w) => w.id === active.id)
      const newIndex = visible.findIndex((w) => w.id === over.id)
      if (oldIndex === -1 || newIndex === -1) return prev
      const reordered = arrayMove(visible, oldIndex, newIndex)
      let next = 0
      return prev.map((w) => (w.visible ? reordered[next++] : w))
    })
  }, [commitDashboard])

  const handleAddWidget = useCallback((config: WidgetConfig, width: WidgetWidth, height: WidgetHeight) => {
    commitDashboard((prev) => [...prev, { ...newWidget(config), width, height }])
  }, [commitDashboard])

  const handleSaveWidgetConfig = useCallback((config: WidgetConfig, width: WidgetWidth, height: WidgetHeight) => {
    if (!editingWidgetId) return
    commitDashboard((prev) => prev.map((w) => (w.id === editingWidgetId ? { ...w, config, width, height } : w)))
  }, [commitDashboard, editingWidgetId])

  const catData = categoryQuery.data

  const renderWidgetBody = useCallback((config: WidgetConfig) => {
    // Self-contained — each has its own year/month(s), doesn't depend on the
    // page-level year filter at all.
    if (config.type === 'monthCompare') {
      return <MonthCompareWidget year={config.year} months={config.months} chartKind={config.chartKind} />
    }
    if (config.type === 'allTimeTrend') {
      return <AllTimeTrendWidget fromYear={config.fromYear} toYear={config.toYear} chartKind={config.chartKind} />
    }
    if (config.type === 'monthAcrossYears') {
      return <MonthAcrossYearsWidget month={config.month} years={config.years} chartKind={config.chartKind} />
    }
    if (!filter || !catData) return null
    if (config.type === 'kpi') return <KpiWidget metric={config.metric} filter={filter} allYears={allYears} />
    return <CategoryWidget categoryIds={config.categoryIds} chartKind={config.chartKind} filter={filter} catData={catData} />
  }, [filter, catData, allYears])

  // Error first: `registeredYears` stays empty when /api/years fails, which
  // used to leave the page on a skeleton the Alert below could never replace.
  if (yearsQuery.error || categoryQuery.error || registeredYearsQuery.error) return <Alert color="red">{t('common.error')}</Alert>
  if (yearsQuery.isLoading || categoryQuery.isLoading || registeredYearsQuery.isLoading) {
    return <Skeleton h={400} />
  }
  if (!catData) return null
  if (dashboard === null || filter === null) return <Skeleton h={400} />

  const visibleWidgets = dashboard.filter((w) => w.visible)
  const hiddenWidgets = dashboard.filter((w) => !w.visible)
  const editingWidget = dashboard.find((w) => w.id === editingWidgetId)
  const yearOptions = registeredYears.length > 0 ? registeredYears.map(String) : [String(filter.year)]

  return (
    <Stack gap="xl">
      <Group justify="space-between" wrap="wrap">
        <Title order={isMobile ? 3 : 2}>{t('trends.title')}</Title>
        <Tooltip label={editMode ? t('trends.doneEditing') : t('trends.editDashboard')}>
          <ActionIcon variant={editMode ? 'filled' : 'subtle'} size="lg" aria-label={editMode ? t('trends.doneEditing') : t('trends.editDashboard')} onClick={() => setEditMode((v) => !v)}>
            {editMode ? <IconCheck size={18} /> : <IconPencil size={18} />}
          </ActionIcon>
        </Tooltip>
      </Group>

      <Group justify="space-between" wrap="wrap" align="flex-end">
        <Select
          label={t('settings.year')}
          data={yearOptions}
          value={String(filter.year)}
          onChange={(v) => v && updateFilter({ year: Number(v) })}
          w={isMobile ? '100%' : 160}
          allowDeselect={false}
        />
        <Button component={Link} to="/trends/months" variant="light" size={isMobile ? 'xs' : 'sm'} leftSection={<IconArrowsLeftRight size={16} />} fullWidth={isMobile}>
          {t('trends.compareMonthsLink')}
        </Button>
      </Group>

      {visibleWidgets.length === 0 ? (
        <EmptyState message={t('trends.noWidgets')} />
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={visibleWidgets.map((w) => w.id)} strategy={rectSortingStrategy}>
            <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }} style={{ gridAutoFlow: 'dense', gridAutoRows: `${ROW_UNIT_PX}px` }}>
              {visibleWidgets.map((w) => (
                <WidgetFrame
                  key={w.id}
                  id={w.id}
                  title={widgetTitle(w.config, catData.categories, t, monthNames)}
                  editMode={editMode}
                  width={Math.min(w.width, maxWidgetWidth) as WidgetWidth}
                  height={w.height}
                  onHide={() => handleSetVisible(w.id, false)}
                  onDelete={() => handleDeleteWidget(w.id)}
                  onConfigure={() => { setEditingWidgetId(w.id); setModalOpened(true) }}
                >
                  {renderWidgetBody(w.config)}
                </WidgetFrame>
              ))}
            </SimpleGrid>
          </SortableContext>
        </DndContext>
      )}

      {editMode && (
        <Stack gap="xs">
          {hiddenWidgets.length > 0 && (
            <Group gap="xs">
              {hiddenWidgets.map((w) => (
                <Group key={w.id} gap={2} wrap="nowrap">
                  <Chip checked={false} icon={<IconEye size={14} />} onChange={() => handleSetVisible(w.id, true)}>
                    {widgetTitle(w.config, catData.categories, t, monthNames)}
                  </Chip>
                  <Tooltip label={t('trends.deleteWidget')}>
                    <ActionIcon variant="subtle" color="red" size="sm" aria-label={t('trends.deleteWidget')} onClick={() => handleDeleteWidget(w.id)}>
                      <IconTrash size={14} />
                    </ActionIcon>
                  </Tooltip>
                </Group>
              ))}
            </Group>
          )}
          <Button
            variant="light"
            leftSection={<IconPlus size={16} />}
            onClick={() => { setEditingWidgetId(null); setModalOpened(true) }}
            style={{ alignSelf: 'flex-start' }}
          >
            {t('trends.addWidget')}
          </Button>
        </Stack>
      )}

      <WidgetModal
        opened={modalOpened}
        onClose={() => setModalOpened(false)}
        onSubmit={editingWidget ? handleSaveWidgetConfig : handleAddWidget}
        categories={catData.categories}
        years={registeredYears}
        initial={editingWidget?.config}
        initialWidth={editingWidget?.width}
        initialHeight={editingWidget?.height}
      />
    </Stack>
  )
}
