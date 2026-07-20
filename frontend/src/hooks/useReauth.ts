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

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  useEffect(() => stopPolling, [stopPolling])

  const start = useCallback(() => {
    setState('pending')
    popupRef.current = window.open('/auth/reauth', 'reauth', 'width=480,height=640')

    pollRef.current = setInterval(async () => {
      if (!popupRef.current || popupRef.current.closed) {
        stopPolling()
        setState((s) => (s === 'fresh' ? s : 'cancelled'))
        return
      }
      try {
        const { fresh } = await api.get<{ fresh: boolean }>('/api/reauth-status')
        if (fresh) {
          stopPolling()
          popupRef.current?.close()
          setState('fresh')
        }
      } catch {
        // transient — keep polling until the popup closes or times out
      }
    }, POLL_INTERVAL_MS)
  }, [stopPolling])

  const reset = useCallback(() => setState('idle'), [])

  return { state, start, reset }
}
