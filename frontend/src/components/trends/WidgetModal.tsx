import { useEffect, useState } from 'react'
import { Modal, Select, SegmentedControl, Stack, Button, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { WidgetConfig, ChartKind } from '../../lib/trendsDashboard'
import type { CategoryTotalsCategory } from '../../api/types'

type BaseType = WidgetConfig['type']

interface Props {
  opened: boolean
  onClose: () => void
  onSubmit: (config: WidgetConfig) => void
  categories: CategoryTotalsCategory[]
  initial?: WidgetConfig
}

export default function WidgetModal({ opened, onClose, onSubmit, categories, initial }: Props) {
  const { t } = useTranslation()
  const [type, setType] = useState<BaseType>(initial?.type ?? 'categoryChart')
  const [metric, setMetric] = useState<string>(initial?.type === 'kpi' ? initial.metric : 'income')
  const [categoryId, setCategoryId] = useState<string | null>(
    initial?.type === 'categoryChart' ? String(initial.categoryId) : (categories[0] ? String(categories[0].id) : null),
  )
  const [chartKind, setChartKind] = useState<ChartKind>(
    initial && initial.type !== 'kpi' ? initial.chartKind : 'line',
  )

  useEffect(() => {
    if (!opened) return
    setType(initial?.type ?? 'categoryChart')
    setMetric(initial?.type === 'kpi' ? initial.metric : 'income')
    setCategoryId(initial?.type === 'categoryChart' ? String(initial.categoryId) : (categories[0] ? String(categories[0].id) : null))
    setChartKind(initial && initial.type !== 'kpi' ? initial.chartKind : 'line')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened, initial])

  const canSubmit = type !== 'categoryChart' || categoryId != null

  const handleSubmit = () => {
    if (type === 'kpi') {
      onSubmit({ type: 'kpi', metric: metric as 'income' | 'expenses' | 'surplus' | 'yearsTracked' })
    } else if (type === 'categoryChart') {
      if (!categoryId) return
      onSubmit({ type: 'categoryChart', categoryId: Number(categoryId), chartKind })
    } else {
      onSubmit({ type: 'yearCompare', chartKind })
    }
    onClose()
  }

  return (
    <Modal opened={opened} onClose={onClose} title={initial ? t('trends.configureWidget') : t('trends.addWidget')}>
      <Stack gap="sm">
        {!initial && (
          <SegmentedControl
            fullWidth
            value={type}
            onChange={(v) => setType(v as BaseType)}
            data={[
              { label: t('trends.widgetType_kpi'), value: 'kpi' },
              { label: t('trends.widgetType_categoryChart'), value: 'categoryChart' },
              { label: t('trends.widgetType_yearCompare'), value: 'yearCompare' },
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

        <Button disabled={!canSubmit} onClick={handleSubmit}>
          {initial ? t('common.save') : t('common.add')}
        </Button>
      </Stack>
    </Modal>
  )
}
