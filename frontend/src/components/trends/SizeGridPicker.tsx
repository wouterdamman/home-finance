import { Stack, Group, Text, Slider } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { WidgetWidth, WidgetHeight } from '../../lib/trendsDashboard'

const COLUMNS: WidgetWidth[] = [1, 2, 3, 4]
const ROWS: WidgetHeight[] = [1, 2, 3, 4]
const CELL = 36
const GAP = 4
const GRID_SIZE = COLUMNS.length * CELL + (COLUMNS.length - 1) * GAP

interface Props {
  width: WidgetWidth
  height: WidgetHeight
  onChange: (width: WidgetWidth, height: WidgetHeight) => void
}

// Mirrors Home Assistant's card-layout picker: a horizontal slider for
// width, a vertical slider for height, and a grid that shows (and can
// also be clicked directly) the same selection — all three stay in sync.
export default function SizeGridPicker({ width, height, onChange }: Props) {
  const { t } = useTranslation()
  return (
    <Stack gap={4}>
      <Text size="sm" fw={500}>{t('trends.size')}</Text>
      <Group gap="xs" align="flex-start" wrap="nowrap">
        <Slider
          orientation="vertical"
          h={GRID_SIZE}
          min={1}
          max={4}
          step={1}
          value={height}
          onChange={(v) => onChange(width, v as WidgetHeight)}
          label={(v) => `${v}`}
          marks={ROWS.map((r) => ({ value: r }))}
        />
        <Stack gap="xs">
          <Slider
            w={GRID_SIZE}
            min={1}
            max={4}
            step={1}
            value={width}
            onChange={(v) => onChange(v as WidgetWidth, height)}
            label={(v) => `${v}`}
            marks={COLUMNS.map((c) => ({ value: c }))}
          />
          <div style={{ display: 'inline-grid', gridTemplateColumns: `repeat(${COLUMNS.length}, ${CELL}px)`, gap: GAP }}>
            {ROWS.map((row) =>
              COLUMNS.map((col) => (
                <button
                  key={`${col}-${row}`}
                  type="button"
                  aria-label={`${col} x ${row}`}
                  onClick={() => onChange(col, row)}
                  style={{
                    width: CELL,
                    height: CELL,
                    borderRadius: 6,
                    border: '1px solid var(--mantine-color-gray-4)',
                    backgroundColor: col <= width && row <= height ? 'var(--mantine-color-teal-6)' : 'var(--mantine-color-gray-1)',
                    cursor: 'pointer',
                    padding: 0,
                  }}
                />
              )),
            )}
          </div>
        </Stack>
      </Group>
      <Text size="xs" c="dimmed">{t('trends.sizeOf', { width, height })}</Text>
    </Stack>
  )
}
