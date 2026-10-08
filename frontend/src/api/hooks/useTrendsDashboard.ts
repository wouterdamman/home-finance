import { useMutation, useQuery } from '@tanstack/react-query'
import { api } from '../client'

interface TrendsDashboardResponse {
  widgets: unknown[] | null
}

export function useServerTrendsDashboard() {
  return useQuery<TrendsDashboardResponse>({
    queryKey: ['trends-dashboard'],
    queryFn: ({ signal }) => api.get<TrendsDashboardResponse>('/api/me/trends-dashboard', { signal }),
    // The page owns the board after the first pull; refetching would only
    // race the user's own unsaved edits.
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  })
}

export function useSaveTrendsDashboard() {
  return useMutation({
    mutationFn: (widgets: unknown[]) => api.put<void>('/api/me/trends-dashboard', { widgets }),
  })
}
