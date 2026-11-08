import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import type { IntegrationToken, CreatedIntegrationToken } from '../types'

export const INTEGRATION_SCOPES = [
  { value: 'kids:read', labelKey: 'settings.scopeKidsRead' },
  { value: 'pots:read', labelKey: 'settings.scopePotsRead' },
] as const

export function useIntegrationTokens() {
  return useQuery<IntegrationToken[]>({
    queryKey: ['integration-tokens'],
    queryFn: ({ signal }) => api.get<IntegrationToken[]>('/api/integration-tokens', { signal }),
  })
}

export function useCreateIntegrationToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; scopes: string[]; expiresAt?: string }) =>
      api.post<CreatedIntegrationToken>('/api/integration-tokens', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['integration-tokens'] }),
  })
}

export function useRevokeIntegrationToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete<void>(`/api/integration-tokens/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['integration-tokens'] }),
  })
}
