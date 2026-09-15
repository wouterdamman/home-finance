import { Group, Text } from '@mantine/core'
import { paletteColorValue } from '../../lib/chartPalette'

interface LegendSeries {
  name: string
  color: string
}

// Recharts' own legend can be pushed into normal document flow (needed to
// make it wrap instead of overflowing its card), but doing so adds its
// height on top of the chart's fixed h={} — the surrounding card was
// already sized without that extra height, so content bled into the next
// grid row. A plain Mantine legend in normal flow avoids that entirely:
// the card's natural height always includes it correctly.
export default function ChartLegend({ series }: { series: LegendSeries[] }) {
  return (
    <Group gap="sm" justify="center" wrap="wrap" mb={4}>
      {series.map((s) => (
        <Group key={s.name} gap={4} wrap="nowrap">
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: paletteColorValue(s.color),
              flexShrink: 0,
            }}
          />
          <Text size="xs" c="dimmed">{s.name}</Text>
        </Group>
      ))}
    </Group>
  )
}
