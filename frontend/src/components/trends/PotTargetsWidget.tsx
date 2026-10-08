import { memo } from 'react'
import { Skeleton, Text, Stack, Group, Progress, Badge } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'
import { useTrendsPotBalances } from '../../api/hooks/usePeriods'
import { formatCents } from '../../lib/money'
import { monthNames } from '../../lib/monthNames'

// How many of the most recent months-with-data to average inflow over for
// the completion projection.
const PROJECTION_MONTHS = 3

// Self-contained, like incomeMix/itemizedIncome — no config fields beyond
// type, always reflects "now" rather than the page's year filter.
function PotTargetsWidget() {
  const { t, i18n } = useTranslation()
  const locale = i18n.language.startsWith('nl') ? 'nl-NL' : 'en-US'
  const months = monthNames(i18n.language)
  const { data, isLoading, error } = useTrendsPotBalances()

  if (isLoading) return <Skeleton h="100%" />
  // A failed request is not the same as an empty result: without this the
  // widget told the user there was nothing to show when the call had in
  // fact errored.
  if (error) return <Text size="sm" c="dimmed">{t('common.error')}</Text>

  // Archived pots are excluded: the endpoint returns them so their history
  // can be charted, but this widget is about goals still being saved for.
  const potsWithTarget = (data?.pots ?? []).filter((p) => p.targetCents != null && p.targetCents > 0 && p.archivedAt == null)
  if (!data || potsWithTarget.length === 0) {
    return <Text size="sm" c="dimmed">{t('trends.noData')}</Text>
  }

  const sortedEntries = [...data.entries].sort((a, b) => (a.year - b.year) || (a.month - b.month))
  const latest = sortedEntries[sortedEntries.length - 1]

  const rows = potsWithTarget.map((pot) => {
    const key = String(pot.id)
    const targetCents = pot.targetCents!
    const balanceCents = latest?.balances[key] ?? 0
    const percent = Math.max(0, Math.min(100, (balanceCents / targetCents) * 100))

    // "Months with data" = months this pot actually has an inflow entry
    // for, newest first, so a pot created partway through the year doesn't
    // average in months before it existed.
    const recentInflows = [...sortedEntries]
      .reverse()
      .filter((e) => key in e.inflow)
      .slice(0, PROJECTION_MONTHS)
      .map((e) => e.inflow[key])
    const avgInflowCents = recentInflows.length > 0
      ? recentInflows.reduce((sum, v) => sum + v, 0) / recentInflows.length
      : 0

    let projectedLabel: string
    let behindSchedule = false
    if (balanceCents >= targetCents) {
      projectedLabel = t('trends.potTargetComplete')
    } else if (avgInflowCents <= 0 || !latest) {
      projectedLabel = t('trends.potTargetNoProjection')
      // A pot with no recent inflow whose target date has already gone by is
      // the most clearly off-track case there is — it used to be the only
      // one that never got the badge, because the flag was set solely in the
      // projection branch below.
      if (pot.targetDate && dayjs(pot.targetDate).isBefore(dayjs(), 'month')) {
        behindSchedule = true
      }
    } else {
      const remainingCents = targetCents - balanceCents
      const monthsNeeded = Math.ceil(remainingCents / avgInflowCents)
      const projectedDate = dayjs(`${latest.year}-${String(latest.month).padStart(2, '0')}-01`).add(monthsNeeded, 'month')
      const dateLabel = `${months[projectedDate.month()]} ${projectedDate.year()}`
      projectedLabel = t('trends.potTargetProjected', { date: dateLabel })
      if (pot.targetDate && projectedDate.isAfter(dayjs(pot.targetDate), 'month')) {
        behindSchedule = true
      }
    }

    return { pot, balanceCents, targetCents, percent, projectedLabel, behindSchedule }
  })

  return (
    <Stack gap="sm" style={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>
      {rows.map(({ pot, balanceCents, targetCents, percent, projectedLabel, behindSchedule }) => (
        <div key={pot.id}>
          <Group justify="space-between" wrap="nowrap" mb={2} gap="xs">
            <Text size="sm" fw={600} truncate>{pot.name}</Text>
            <Text size="xs" c="dimmed" style={{ flexShrink: 0 }}>{t('pots.targetProgress', { percent: Math.floor(percent) })}</Text>
          </Group>
          <Progress value={percent} color={percent >= 100 ? 'green' : 'blue'} size="sm" />
          <Group justify="space-between" wrap="wrap" mt={2} gap="xs">
            <Text size="xs" c="dimmed">
              {formatCents(balanceCents, locale)} / {formatCents(targetCents, locale)}
            </Text>
            <Group gap={4} wrap="nowrap">
              <Text size="xs" c="dimmed">{projectedLabel}</Text>
              {/* Status colors are reserved for state like this, never reused
                  as a categorical series color — always icon/label + color. */}
              {behindSchedule && <Badge color="red" size="xs" variant="light">{t('trends.potTargetBehind')}</Badge>}
            </Group>
          </Group>
        </div>
      ))}
    </Stack>
  )
}

// Memoized so toggling edit mode / opening the config modal / changing the
// page's year Select doesn't re-render and re-lay-out every chart on the grid.
export default memo(PotTargetsWidget)
