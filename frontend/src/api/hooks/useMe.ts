import { useQuery } from '@tanstack/react-query'
import { api, isUnauthorizedError } from '../client'
import type { User } from '../types'

export function useMe() {
  return useQuery<User | null>({
    queryKey: ['me'],
    // Only "not signed in" may resolve to null — swallowing every failure here
    // turns a backend outage into a sign-in screen and makes `retry` dead.
    queryFn: async ({ signal }) => {
      try {
        return await api.get<User>('/api/me', { signal })
      } catch (err) {
        if (isUnauthorizedError(err)) return null
        throw err
      }
    },
    staleTime: 5 * 60 * 1000,
  })
}
