import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, cleanup, fireEvent, within } from '@testing-library/react'
import { MantineProvider } from '@mantine/core'
import dayjs from 'dayjs'
import { ModalsProvider } from '@mantine/modals'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import IntegrationsTab from './IntegrationsTab'

vi.mock('react-i18next', () => ({
  initReactI18next: { type: '3rdParty', init: () => {} },
  useTranslation: () => ({ t: (key: string) => key }),
}))

// The real picker is a calendar popover; a plain input lets the test set an
// arbitrary value and exercise the range check.
vi.mock('@mantine/dates', () => ({
  DateTimePicker: ({ label, value, onChange }: { label: string; value: string | null; onChange: (v: string | null) => void }) => (
    <input aria-label={label} value={value ?? ''} onChange={(e) => onChange(e.target.value || null)} />
  ),
}))

const baseToken = {
  scopes: ['kids:read'],
  createdAt: '2026-11-01T10:00:00Z',
  createdByEmail: 'admin@example.com',
}
const active = { ...baseToken, id: 2, name: 'Home Assistant', tokenPrefix: 'hf_ab12', lastUsedAt: null, revokedAt: null, expiresAt: dayjs().add(1, 'year').toISOString() }
const revoked = { ...baseToken, id: 1, name: 'Old script', tokenPrefix: 'hf_zz99', lastUsedAt: '2026-11-02T08:00:00Z', revokedAt: '2026-11-03T08:00:00Z', expiresAt: dayjs().add(1, 'year').toISOString() }
const expired = { ...baseToken, id: 3, name: 'Lapsed script', tokenPrefix: 'hf_ex00', lastUsedAt: null, revokedAt: null, expiresAt: '2026-11-04T08:00:00Z' }

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

function renderTab() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MantineProvider>
        <ModalsProvider>
          <IntegrationsTab />
        </ModalsProvider>
      </MantineProvider>
    </QueryClientProvider>,
  )
}

describe('IntegrationsTab', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: false, media: query, onchange: null,
      addListener: () => {}, removeListener: () => {},
      addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => false,
    }))
    Element.prototype.scrollIntoView = vi.fn()
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('renders active and revoked tokens', async () => {
    fetchMock.mockResolvedValue(json([active, revoked]))
    renderTab()
    expect(await screen.findByText('Home Assistant')).toBeTruthy()
    expect(screen.getByText('Old script')).toBeTruthy()
    expect(screen.getByText('hf_ab12…')).toBeTruthy()
    expect(screen.getByText('settings.integrationsActive')).toBeTruthy()
    expect(screen.getByText('settings.integrationsRevoked')).toBeTruthy()
    expect(screen.getByText('settings.integrationsNever')).toBeTruthy()
    expect(screen.getAllByText('settings.integrationsRevoke')).toHaveLength(1)
  })

  it('shows the empty state', async () => {
    fetchMock.mockResolvedValue(json([]))
    renderTab()
    expect(await screen.findByText('settings.integrationsEmpty')).toBeTruthy()
  })

  it('requires a name and a scope before submitting', async () => {
    fetchMock.mockResolvedValue(json([]))
    renderTab()
    fireEvent.click(await screen.findByText('settings.integrationsNewToken'))
    const submit = (await screen.findByText('settings.integrationsCreate')).closest('button')!
    expect(submit.disabled).toBe(true)
    fireEvent.change(screen.getByLabelText(/settings.integrationsName/), { target: { value: 'HA' } })
    expect(submit.disabled).toBe(true)
    fireEvent.click(screen.getByLabelText('settings.scopeKidsRead'))
    expect(submit.disabled).toBe(false)
    fireEvent.change(screen.getByLabelText(/settings.integrationsName/), { target: { value: '' } })
    expect(submit.disabled).toBe(true)
  })

  it('shows the raw token once after creation and clears it on close', async () => {
    fetchMock.mockImplementation((_url: string, init?: RequestInit) => {
      if (init?.method === 'POST') return Promise.resolve(json({ ...active, token: 'hf_ab12_secretvalue' }, 201))
      return Promise.resolve(json([]))
    })
    renderTab()
    fireEvent.click(await screen.findByText('settings.integrationsNewToken'))
    fireEvent.change(await screen.findByLabelText(/settings.integrationsName/), { target: { value: 'Home Assistant' } })
    fireEvent.click(screen.getByLabelText('settings.scopeKidsRead'))
    fireEvent.click(screen.getByText('settings.integrationsCreate'))

    const field = await screen.findByDisplayValue('hf_ab12_secretvalue')
    expect((field as HTMLInputElement).readOnly).toBe(true)
    expect(screen.getByText('settings.integrationsTokenWarning')).toBeTruthy()
    expect(screen.getByText('settings.integrationsCopy')).toBeTruthy()

    const post = fetchMock.mock.calls.find(c => c[1]?.method === 'POST')!
    const sent = JSON.parse(post[1].body)
    expect(sent.name).toBe('Home Assistant')
    expect(sent.scopes).toEqual(['kids:read'])
    const diffDays = dayjs(sent.expiresAt).diff(dayjs(), 'day', true)
    expect(diffDays).toBeGreaterThan(364)
    expect(diffDays).toBeLessThan(367)
    expect(screen.getByText('settings.integrationsExpiresOn')).toBeTruthy()

    fireEvent.click(screen.getByText('settings.integrationsDone'))
    await waitFor(() => expect(screen.queryByDisplayValue('hf_ab12_secretvalue')).toBeNull())
    fireEvent.click(screen.getByText('settings.integrationsNewToken'))
    expect(await screen.findByLabelText(/settings.integrationsName/)).toBeTruthy()
    expect(screen.queryByDisplayValue('hf_ab12_secretvalue')).toBeNull()
  })

  it('defaults the expiry to 1 year', async () => {
    fetchMock.mockResolvedValue(json([]))
    renderTab()
    fireEvent.click(await screen.findByText('settings.integrationsNewToken'))
    const select = await screen.findByRole('combobox', { name: 'settings.integrationsExpires' })
    expect((select as HTMLInputElement).value).toBe('settings.integrationsExpiry1y')
  })

  it('disables submit for an empty or out-of-range custom expiry', async () => {
    fetchMock.mockResolvedValue(json([]))
    renderTab()
    fireEvent.click(await screen.findByText('settings.integrationsNewToken'))
    fireEvent.change(await screen.findByLabelText(/settings.integrationsName/), { target: { value: 'HA' } })
    fireEvent.click(screen.getByLabelText('settings.scopeKidsRead'))
    const submit = screen.getByText('settings.integrationsCreate').closest('button')!
    expect(submit.disabled).toBe(false)

    fireEvent.click(screen.getByRole('combobox', { name: 'settings.integrationsExpires' }))
    fireEvent.click(await screen.findByText('settings.integrationsExpiryCustom'))
    const picker = await screen.findByLabelText('settings.integrationsExpiryCustomLabel')
    expect(submit.disabled).toBe(true)

    fireEvent.change(picker, { target: { value: dayjs().add(30, 'minute').format('YYYY-MM-DD HH:mm:ss') } })
    expect(submit.disabled).toBe(true)
    fireEvent.change(picker, { target: { value: dayjs().add(3, 'year').format('YYYY-MM-DD HH:mm:ss') } })
    expect(submit.disabled).toBe(true)
    fireEvent.change(picker, { target: { value: dayjs().add(10, 'day').format('YYYY-MM-DD HH:mm:ss') } })
    expect(submit.disabled).toBe(false)
  })

  it('shows an expired token as Expired without a revoke button', async () => {
    fetchMock.mockResolvedValue(json([expired]))
    renderTab()
    expect(await screen.findByText('Lapsed script')).toBeTruthy()
    expect(screen.getByText('settings.integrationsExpired')).toBeTruthy()
    expect(screen.queryByText('settings.integrationsActive')).toBeNull()
    expect(screen.queryByText('settings.integrationsRevoke')).toBeNull()
    const row = screen.getByText('Lapsed script').closest('tr')!
    expect(within(row).getByText('04-11-2026', { exact: false })).toBeTruthy()
  })
})
