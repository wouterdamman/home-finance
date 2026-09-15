import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import { invalidatePeriodAggregates } from '../../lib/queryInvalidation'
import type { Transaction } from '../types'

export function useTransactions(periodId: number, categoryId?: number) {
  const qs = categoryId != null ? `?categoryId=${categoryId}` : ''
  return useQuery<Transaction[]>({
    queryKey: ['transactions', periodId, categoryId],
    queryFn: () => api.get<Transaction[]>(`/api/periods/${periodId}/transactions${qs}`),
    enabled: periodId > 0,
  })
}

export function useCreateTransaction(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { categoryId: number; amountCents: number; description: string; txDate?: string }) =>
      api.post<Transaction>(`/api/periods/${periodId}/transactions`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions', periodId] })
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['category-transaction-descriptions'] })
      invalidatePeriodAggregates(qc)
    },
  })
}

export function useUpdateTransaction(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; amountCents: number; description: string; txDate?: string }) =>
      api.put(`/api/transactions/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions', periodId] })
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['category-transaction-descriptions'] })
      invalidatePeriodAggregates(qc)
    },
  })
}

export function useDeleteTransaction(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/transactions/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transactions', periodId] })
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['category-transaction-descriptions'] })
      invalidatePeriodAggregates(qc)
    },
  })
}

export function useCategories() {
  return useQuery({
    queryKey: ['categories'],
    queryFn: () => api.get<{ id: number; name: string; defaultAmountCents: number; isItemized: boolean; sortOrder: number }[]>('/api/categories'),
  })
}
