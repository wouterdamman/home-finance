import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'

export type ReauthState = 'idle' | 'pending' | 'fresh' | 'cancelled'

const POLL_INTERVAL_MS = 1000

// Drives the Authentik step-up reauth popup used to confirm identity before
// a destructive action. Opening a popup (rather than a full-page redirect)
// keeps the current page's in-progress state intact — e.g. a staged import
// file survives a delete-period confirmation happening elsewhere in the app.
export function useReauth() {
  const [state, setState] = useState<ReauthState>('idle')
  const popupRef = useRef<Window | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  // Bumped by reset()/start() so a poll tick whose /reauth-status request was
  // already in flight when the user cancelled can never resolve into 'fresh'
  // and re-fire the destructive action the dialog was dismissed to avoid.
  const runRef = useRef(0)

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  const closePopup = useCallback(() => {
    const popup = popupRef.current
    popupRef.current = null
    try {
      if (popup && !popup.closed) popup.close()
    } catch {
      // cross-origin popup that navigated away — nothing we can do
    }
  }, [])

  useEffect(() => () => {
    runRef.current += 1
    stopPolling()
  }, [stopPolling])

  const start = useCallback(() => {
    stopPolling()
    closePopup()
    const runId = ++runRef.current
    setState('pending')
    popupRef.current = window.open('/auth/reauth', 'reauth', 'width=480,height=640')

    pollRef.current = setInterval(async () => {
      if (runRef.current !== runId) return
      if (!popupRef.current || popupRef.current.closed) {
        stopPolling()
        setState((s) => (s === 'fresh' ? s : 'cancelled'))
        return
      }
      try {
        const { fresh } = await api.get<{ fresh: boolean }>('/api/reauth-status')
        if (runRef.current !== runId) return
        if (fresh) {
          stopPolling()
          closePopup()
          setState('fresh')
        }
      } catch {
        // transient — keep polling until the popup closes or times out
      }
    }, POLL_INTERVAL_MS)
  }, [stopPolling, closePopup])

  const reset = useCallback(() => {
    runRef.current += 1
    stopPolling()
    closePopup()
    setState('idle')
  }, [stopPolling, closePopup])

  return { state, start, reset }
}
