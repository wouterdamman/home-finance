import type { ReactNode } from 'react'
import { Paper, Group, Text, ActionIcon, Tooltip } from '@mantine/core'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { IconGripVertical, IconEyeOff, IconSettings } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import type { WidgetWidth, WidgetHeight } from '../../lib/trendsDashboard'

interface Props {
  id: string
  title: string
  editMode: boolean
  width: WidgetWidth
  height: WidgetHeight
  onHide: () => void
  onConfigure?: () => void
  children: ReactNode
}

export default function WidgetFrame({ id, title, editMode, width, height, onHide, onConfigure, children }: Props) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id, disabled: !editMode })
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    gridColumn: `span ${width}`,
    gridRow: `span ${height}`,
  }

  return (
    <Paper ref={setNodeRef} style={{ ...style, height: '100%', display: 'flex', flexDirection: 'column' }} shadow="xs" p="sm" withBorder>
      <Group justify="space-between" mb={4} wrap="nowrap" style={{ flexShrink: 0 }}>
        <Group gap={4} wrap="nowrap" style={{ minWidth: 0 }}>
          {editMode && (
            <ActionIcon variant="subtle" color="gray" size="sm" style={{ touchAction: 'none', cursor: 'grab' }} aria-label={t('trends.dragHandle')} {...attributes} {...listeners}>
              <IconGripVertical size={16} />
            </ActionIcon>
          )}
          <Text size="sm" fw={600} truncate>{title}</Text>
        </Group>
        {editMode && (
          <Group gap={2} wrap="nowrap">
            {onConfigure && (
              <Tooltip label={t('trends.configureWidget')}>
                <ActionIcon variant="subtle" color="gray" size="sm" aria-label={t('trends.configureWidget')} onClick={onConfigure}>
                  <IconSettings size={16} />
                </ActionIcon>
              </Tooltip>
            )}
            <Tooltip label={t('trends.hideWidget')}>
              <ActionIcon variant="subtle" color="gray" size="sm" aria-label={t('trends.hideWidget')} onClick={onHide}>
                <IconEyeOff size={16} />
              </ActionIcon>
            </Tooltip>
          </Group>
        )}
      </Group>
      <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
        {children}
      </div>
    </Paper>
  )
}
