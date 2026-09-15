import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act, cleanup } from '@testing-library/react'
import { MantineProvider } from '@mantine/core'
import ReauthConfirmModal from './ReauthConfirmModal'

const freshResponse = vi.fn<() => boolean>(() => false)

vi.mock('../api/client', () => ({
  api: { get: () => Promise.resolve({ fresh: freshResponse() }) },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function renderModal(props: { opened: boolean; onConfirm: () => void; onClose: () => void }) {
  return render(
    <MantineProvider>
      <ReauthConfirmModal
        opened={props.opened}
        onClose={props.onClose}
        title="Delete period"
        warningText="This cannot be undone"
        confirmLabel="Delete"
        loading={false}
        onConfirm={props.onConfirm}
      />
    </MantineProvider>,
  )
}

// Lets the 1s interval fire and its awaited fetch settle.
async function tick(ms: number) {
  await act(async () => {
    vi.advanceTimersByTime(ms)
    await Promise.resolve()
    await Promise.resolve()
  })
}

describe('ReauthConfirmModal', () => {
  let popup: { closed: boolean; close: ReturnType<typeof vi.fn> }

  beforeEach(() => {
    vi.useFakeTimers()
    freshResponse.mockReturnValue(false)
    popup = { closed: false, close: vi.fn(() => { popup.closed = true }) }
    vi.stubGlobal('open', vi.fn(() => popup))
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('does not confirm after the dialog is cancelled, even if the popup later succeeds', async () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    const { rerender } = renderModal({ opened: true, onConfirm, onClose })

    await act(async () => {
      screen.getByRole('button', { name: 'common.verifyIdentity' }).click()
    })
    await tick(1000)
    expect(onConfirm).not.toHaveBeenCalled()

    // Dismiss with the header X — the Cancel button is gone once polling starts.
    const closeButton = document.querySelector<HTMLButtonElement>('.mantine-Modal-close')
    expect(closeButton).not.toBeNull()
    await act(async () => {
      closeButton!.click()
    })
    expect(onClose).toHaveBeenCalled()
    expect(popup.close).toHaveBeenCalled()

    // Call sites keep the modal mounted; only `opened` flips.
    rerender(
      <MantineProvider>
        <ReauthConfirmModal
          opened={false}
          onClose={onClose}
          title="Delete period"
          warningText="This cannot be undone"
          confirmLabel="Delete"
          loading={false}
          onConfirm={onConfirm}
        />
      </MantineProvider>,
    )

    freshResponse.mockReturnValue(true)
    await tick(5000)

    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('confirms once when the popup completes while the dialog is still open', async () => {
    const onConfirm = vi.fn()
    renderModal({ opened: true, onConfirm, onClose: vi.fn() })

    await act(async () => {
      screen.getByRole('button', { name: 'common.verifyIdentity' }).click()
    })
    freshResponse.mockReturnValue(true)
    await tick(1000)

    expect(onConfirm).toHaveBeenCalledTimes(1)

    await tick(5000)
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })
})
