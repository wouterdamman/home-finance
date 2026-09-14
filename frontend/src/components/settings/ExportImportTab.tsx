import { useState } from 'react'
import { Alert, Button, Divider, FileInput, MultiSelect, NumberInput, SegmentedControl, Select, Stack, Switch, Table, Text } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import { IconAlertTriangle, IconDownload, IconUpload } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { useYears, useImportXLSX } from '../../api/hooks/usePeriods'
import { useCurrentYear } from '../../api/hooks/useCurrentYear'
import { getErrorMessage } from '../../api/client'
import type { ImportReport } from '../../api/types'
import MoneyText from '../MoneyText'
import ReauthConfirmModal from '../ReauthConfirmModal'

const EXPORT_MONTH_NL = ['', 'Januari', 'Februari', 'Maart', 'April', 'Mei', 'Juni', 'Juli', 'Augustus', 'September', 'Oktober', 'November', 'December']
const EXPORT_MONTH_EN = ['', 'January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

export default function ExportImportTab() {
  const { t, i18n } = useTranslation()
  const { data: years } = useYears()
  const currentYear = useCurrentYear()
  // useCurrentYear() answers with the calendar year until useYears() resolves,
  // so seeding state from it freezes a year that may not exist on this install.
  const [pickedYear, setPickedYear] = useState<number | null>(null)
  const year = pickedYear ?? currentYear
  const [scope, setScope] = useState<'year' | 'months'>('year')
  const [months, setMonths] = useState<string[]>([])

  const monthNames = i18n.language.startsWith('nl') ? EXPORT_MONTH_NL : EXPORT_MONTH_EN
  const monthOptions = monthNames.slice(1).map((label, i) => ({ value: String(i + 1), label }))

  const handleDownload = () => {
    const query = scope === 'months' && months.length > 0 ? `?months=${months.join(',')}` : ''
    const a = document.createElement('a')
    a.href = `/api/export/years/${year}${query}`
    document.body.appendChild(a)
    a.click()
    a.remove()
    notifications.show({ color: 'green', message: t('export.downloadStarted') })
  }

  return (
    <Stack gap="md" maw={420}>
      <Text size="sm" c="dimmed">{t('export.description')}</Text>
      <Select
        label={t('settings.year')}
        data={(years ?? []).map(String)}
        value={String(year)}
        onChange={v => v && setPickedYear(Number(v))}
      />
      <Stack gap="xs">
        <Text size="sm" fw={600}>{t('export.scope')}</Text>
        <SegmentedControl
          value={scope}
          onChange={v => setScope(v as 'year' | 'months')}
          fullWidth
          data={[
            { label: t('export.scopeWholeYear'), value: 'year' },
            { label: t('export.scopeMonths'), value: 'months' },
          ]}
        />
      </Stack>
      {scope === 'months' && (
        <MultiSelect
          label={t('export.months')}
          placeholder={t('export.scopeMonths')}
          data={monthOptions}
          value={months}
          onChange={setMonths}
        />
      )}
      <Button
        leftSection={<IconDownload size={16} />}
        disabled={scope === 'months' && months.length === 0}
        onClick={handleDownload}
      >
        {t('export.download')}
      </Button>

      <Divider my="sm" />

      <ImportSection />
    </Stack>
  )
}

function ImportSection() {
  const { t } = useTranslation()
  const currentYear = useCurrentYear()
  const importMutation = useImportXLSX()
  const [file, setFile] = useState<File | null>(null)
  const [pickedImportYear, setPickedImportYear] = useState<number | string | null>(null)
  const importYear = pickedImportYear ?? currentYear
  const [wipe, setWipe] = useState(false)
  const [resetMaster, setResetMaster] = useState(false)
  const [closeThrough, setCloseThrough] = useState<number | string>(0)
  const [pwOpened, { open: openPw, close: closePw }] = useDisclosure(false)
  const [report, setReport] = useState<ImportReport | null>(null)

  const runImport = () => {
    if (!file || !importYear) return
    importMutation.mutate({
      file,
      year: Number(importYear),
      wipe,
      resetMaster,
      closeThrough: Number(closeThrough) || 0,
    }, {
      onSuccess: (data) => {
        setReport(data)
        closePw()
        if (data.skippedSheets?.length) {
          notifications.show({ color: 'yellow', message: t('export.skippedSheets') + ' ' + data.skippedSheets.join(', ') })
        } else {
          notifications.show({ color: 'green', message: t('export.importSuccess') })
        }
      },
      onError: (err: unknown) => {
        notifications.show({ color: 'red', title: t('common.error'), message: getErrorMessage(err, t('common.error')) })
      },
    })
  }

  const handleImportClick = () => {
    if (wipe || resetMaster) {
      openPw()
      return
    }
    runImport()
  }

  return (
    <Stack gap="md">
      <Text size="sm" fw={600}>{t('export.importTitle')}</Text>
      <Text size="sm" c="dimmed">{t('export.importDescription')}</Text>
      <FileInput
        label={t('export.file')}
        placeholder={t('export.filePlaceholder')}
        leftSection={<IconUpload size={16} />}
        accept=".xlsx"
        value={file}
        onChange={setFile}
        clearable
      />
      <NumberInput
        label={t('settings.year')}
        value={importYear}
        onChange={setPickedImportYear}
        hideControls
        decimalScale={0}
      />
      <NumberInput
        label={t('export.closeThrough')}
        description={t('export.closeThroughHint')}
        value={closeThrough}
        onChange={setCloseThrough}
        min={0}
        max={12}
        hideControls
      />
      <Switch
        label={t('export.wipe')}
        description={t('export.wipeHint')}
        checked={wipe}
        onChange={e => setWipe(e.target.checked)}
      />
      <Switch
        color="red"
        label={t('export.resetMaster')}
        description={t('export.resetMasterHint')}
        checked={resetMaster}
        onChange={e => setResetMaster(e.target.checked)}
      />
      <Button
        leftSection={<IconUpload size={16} />}
        color={wipe || resetMaster ? 'red' : undefined}
        disabled={!file || !importYear}
        loading={importMutation.isPending}
        onClick={handleImportClick}
      >
        {t('export.importButton')}
      </Button>

      {report?.skippedSheets && report.skippedSheets.length > 0 && (
        <Alert color="yellow" icon={<IconAlertTriangle size={16} />} title={t('export.skippedSheets')}>
          {report.skippedSheets.join(', ')}
        </Alert>
      )}

      {report?.problems && report.problems.length > 0 && (
        <Alert color="yellow" icon={<IconAlertTriangle size={16} />} title={t('export.importProblems')}>
          <Stack gap={2}>
            {report.problems.map((p, i) => (
              <Text key={i} size="sm">{p}</Text>
            ))}
          </Stack>
        </Alert>
      )}

      {report?.resetCounts && Object.keys(report.resetCounts).length > 0 && (
        <Alert color="orange" icon={<IconAlertTriangle size={16} />} title={t('export.resetCounts')}>
          <Text size="sm">
            {Object.entries(report.resetCounts)
              .filter(([, n]) => n > 0)
              .map(([table, n]) => `${table}: ${n}`)
              .join(' · ')}
          </Text>
        </Alert>
      )}

      {report && (
        <Table mt="sm">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t('export.month')}</Table.Th>
              <Table.Th ta="right">{t('export.income')}</Table.Th>
              <Table.Th ta="right">{t('export.expense')}</Table.Th>
              <Table.Th ta="right">{t('export.surplus')}</Table.Th>
              <Table.Th>{t('common.closed')}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {report.months.map(m => (
              <Table.Tr key={m.month}>
                <Table.Td>{m.month}</Table.Td>
                <Table.Td ta="right"><MoneyText cents={m.incomeTotalCents} size="sm" /></Table.Td>
                <Table.Td ta="right"><MoneyText cents={m.expenseTotalCents} size="sm" /></Table.Td>
                <Table.Td ta="right"><MoneyText cents={m.surplusCents} size="sm" /></Table.Td>
                <Table.Td>{m.closed ? '✓' : ''}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}

      <ReauthConfirmModal
        opened={pwOpened}
        onClose={closePw}
        title={t('export.confirmDestructive')}
        warningText={resetMaster ? t('export.resetMasterWarning') : t('export.wipeWarning')}
        confirmLabel={t('export.importButton')}
        loading={importMutation.isPending}
        onConfirm={runImport}
      />
    </Stack>
  )
}
