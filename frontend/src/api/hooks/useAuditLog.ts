import { useInfiniteQuery } from '@tanstack/react-query'
import { api } from '../client'
import type { AuditLogEntry } from '../types'

export interface AuditLogFilters {
  action?: string
  entityType?: string
  userEmail?: string
  from?: string | null
  to?: string | null
}

const PAGE_SIZE = 50

export function useAuditLog(filters: AuditLogFilters) {
  return useInfiniteQuery({
    queryKey: ['audit-log', filters],
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams()
      if (filters.action) params.set('action', filters.action)
      if (filters.entityType) params.set('entityType', filters.entityType)
      if (filters.userEmail) params.set('userEmail', filters.userEmail)
      if (filters.from) params.set('from', filters.from)
      if (filters.to) params.set('to', filters.to)
      params.set('limit', String(PAGE_SIZE))
      if (pageParam) params.set('before', String(pageParam))
      return api.get<AuditLogEntry[]>(`/api/audit-log?${params.toString()}`)
    },
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (lastPage) => lastPage.length === PAGE_SIZE ? lastPage[lastPage.length - 1].id : undefined,
  })
}
