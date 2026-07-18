import { useEffect, useMemo, useState } from 'react'
import { Title, Skeleton, Alert, Text, Stack, SimpleGrid, Group, SegmentedControl, Select, MultiSelect, ActionIcon, Tooltip, Button, Chip } from '@mantine/core'
import { useMediaQuery } from '@mantine/hooks'
import { useTranslation } from 'react-i18next'
import { IconPencil, IconCheck, IconPlus, IconEye } from '@tabler/icons-react'
import {
  DndContext, closestCenter, PointerSensor, TouchSensor, useSensor, useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { SortableContext, rectSortingStrategy, arrayMove } from '@dnd-kit/sortable'
import { useTrendsYears, useTrendsCategoryTotals, useYears } from '../api/hooks/usePeriods'
import type { TrendsFilter } from '../lib/trendsFilter'
import { MAX_COMPARE_YEARS, loadTrendsFilter, saveTrendsFilter } from '../lib/trendsFilter'
import type { Widget, WidgetConfig } from '../lib/trendsDashboard'
import { loadDashboard, saveDashboard, defaultWidgets, newWidget } from '../lib/trendsDashboard'
import type { CategoryTotalsCategory } from '../api/types'
import WidgetFrame from '../components/trends/WidgetFrame'
import KpiWidget from '../components/trends/KpiWidget'
import CategoryWidget from '../components/trends/CategoryWidget'
import YearCompareWidget from '../components/trends/YearCompareWidget'
import WidgetModal from '../components/trends/WidgetModal'
import EmptyState from '../components/EmptyState'

function widgetTitle(config: WidgetConfig, categories: CategoryTotalsCategory[], t: (key: string) => string): string {
  if (config.type === 'kpi') return t(`trends.metric_${config.metric}`)
  if (config.type === 'yearCompare') return t('trends.yearsTitle')
  return categories.find((c) => c.id === config.categoryId)?.name ?? t('trends.unknownCategory')
}

export default function Trends() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 47.99em)')
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
  // just render as zero everywhere (KpiWidget/CategoryWidget/YearCompare
  // already default missing data to 0).
  const registeredYears = useMemo(() => registeredYearsQuery.data ?? [], [registeredYearsQuery.data])
  const allYears = useMemo(() => yearsQuery.data ?? [], [yearsQuery.data])
  // Years that actually have data — used only to pick sensible defaults
  // (which year to preselect), not to filter what's selectable.
  const yearsWithData = useMemo(
    () => allYears.filter((y) => y.incomeTotalCents !== 0 || y.expenseTotalCents !== 0).map((y) => y.year),
    [allYears],
  )
  const defaultYearPool = registeredYears.length > 0 ? registeredYears : yearsWithData

  useEffect(() => {
    if (dashboard === null && categoryQuery.data) {
      const generated = defaultWidgets(categoryQuery.data.categories)
      setDashboard(generated)
      saveDashboard(generated)
    }
  }, [dashboard, categoryQuery.data])

  useEffect(() => {
    if (filter === null && registeredYears.length > 0) {
      setFilter(loadTrendsFilter(registeredYears))
    }
  }, [filter, registeredYears])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 150, tolerance: 5 } }),
  )

  if (yearsQuery.isLoading || categoryQuery.isLoading || registeredYearsQuery.isLoading || dashboard === null || filter === null) {
    return <Skeleton h={400} />
  }
  if (yearsQuery.error || categoryQuery.error || registeredYearsQuery.error) return <Alert color="red">{t('common.error')}</Alert>
  if (!categoryQuery.data) return null

  const catData = categoryQuery.data

  const updateFilter = (next: TrendsFilter) => {
    setFilter(next)
    saveTrendsFilter(next)
  }

  const updateDashboard = (next: Widget[]) => {
    setDashboard(next)
    saveDashboard(next)
  }

  const visibleWidgets = dashboard.filter((w) => w.visible)
  const hiddenWidgets = dashboard.filter((w) => !w.visible)

  const handleSetVisible = (id: string, visible: boolean) => {
    updateDashboard(dashboard.map((w) => (w.id === id ? { ...w, visible } : w)))
  }

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const oldIndex = visibleWidgets.findIndex((w) => w.id === active.id)
    const newIndex = visibleWidgets.findIndex((w) => w.id === over.id)
    if (oldIndex === -1 || newIndex === -1) return
    const reordered = arrayMove(visibleWidgets, oldIndex, newIndex)
    updateDashboard([...reordered, ...hiddenWidgets])
  }

  const handleAddWidget = (config: WidgetConfig) => {
    updateDashboard([...dashboard, newWidget(config)])
  }

  const handleSaveWidgetConfig = (config: WidgetConfig) => {
    if (!editingWidgetId) return
    updateDashboard(dashboard.map((w) => (w.id === editingWidgetId ? { ...w, config } : w)))
  }

  const handleYearClick = (year: number) => {
    updateFilter({ mode: 'single', year })
  }

  const editingWidget = dashboard.find((w) => w.id === editingWidgetId)

  const renderWidgetBody = (config: WidgetConfig) => {
    if (filter.mode === 'compare' && filter.years.length === 0) {
      return <Text size="sm" c="dimmed">{t('trends.selectYearsHint')}</Text>
    }
    if (config.type === 'kpi') return <KpiWidget metric={config.metric} filter={filter} allYears={allYears} />
    if (config.type === 'categoryChart') return <CategoryWidget categoryId={config.categoryId} chartKind={config.chartKind} filter={filter} catData={catData} />
    return <YearCompareWidget chartKind={config.chartKind} allYears={allYears} onYearClick={handleYearClick} />
  }

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

      <Stack gap="xs">
        <SegmentedControl
          value={filter.mode}
          onChange={(v) => {
            if (v === 'single') {
              updateFilter({ mode: 'single', year: defaultYearPool[defaultYearPool.length - 1] ?? new Date().getFullYear() })
            } else {
              const years = defaultYearPool.slice(-2)
              updateFilter({ mode: 'compare', years })
            }
          }}
          data={[
            { label: t('trends.modeSingle'), value: 'single' },
            { label: t('trends.modeCompare'), value: 'compare' },
          ]}
          fullWidth={isMobile}
          style={{ maxWidth: isMobile ? undefined : 320 }}
        />
        {filter.mode === 'single' ? (
          <Select
            data={registeredYears.map(String)}
            value={String(filter.year)}
            onChange={(v) => v && updateFilter({ mode: 'single', year: Number(v) })}
            w={isMobile ? '100%' : 160}
            allowDeselect={false}
          />
        ) : (
          <MultiSelect
            data={registeredYears.map(String)}
            value={filter.years.map(String)}
            onChange={(vs) => updateFilter({ mode: 'compare', years: vs.map(Number) })}
            maxValues={MAX_COMPARE_YEARS}
            placeholder={t('trends.selectYearsHint')}
            w={isMobile ? '100%' : 320}
          />
        )}
      </Stack>

      {visibleWidgets.length === 0 ? (
        <EmptyState message={t('trends.noWidgets')} />
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={visibleWidgets.map((w) => w.id)} strategy={rectSortingStrategy}>
            <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }}>
              {visibleWidgets.map((w) => (
                <WidgetFrame
                  key={w.id}
                  id={w.id}
                  title={widgetTitle(w.config, catData.categories, t)}
                  editMode={editMode}
                  onHide={() => handleSetVisible(w.id, false)}
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
                <Chip key={w.id} checked={false} icon={<IconEye size={14} />} onChange={() => handleSetVisible(w.id, true)}>
                  {widgetTitle(w.config, catData.categories, t)}
                </Chip>
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
        initial={editingWidget?.config}
      />
    </Stack>
  )
}
