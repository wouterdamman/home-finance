import { useQuery } from '@tanstack/react-query'
import { api } from '../client'
import type { User } from '../types'

export function useMe() {
  return useQuery<User | null>({
    queryKey: ['me'],
    queryFn: () => api.get<User>('/api/me').catch(() => null),
    staleTime: 5 * 60 * 1000,
  })
}
