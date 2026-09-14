import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import { invalidatePeriodAggregates } from '../../lib/queryInvalidation'
import type { IncomeTransaction } from '../types'

export function useIncomeTransactions(periodId: number, sourceId?: number) {
  const qs = sourceId != null ? `?sourceId=${sourceId}` : ''
  return useQuery<IncomeTransaction[]>({
    queryKey: ['income-transactions', periodId, sourceId],
    queryFn: () => api.get<IncomeTransaction[]>(`/api/periods/${periodId}/income-transactions${qs}`),
    enabled: periodId > 0,
  })
}

export function useCreateIncomeTransaction(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { sourceId: number; amountCents: number; description: string; txDate?: string }) =>
      api.post<IncomeTransaction>(`/api/periods/${periodId}/income-transactions`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['income-transactions', periodId] })
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['income-source-transaction-descriptions'] })
      invalidatePeriodAggregates(qc)
    },
  })
}

export function useUpdateIncomeTransaction(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; amountCents: number; description: string; txDate?: string }) =>
      api.put(`/api/income-transactions/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['income-transactions', periodId] })
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['income-source-transaction-descriptions'] })
      invalidatePeriodAggregates(qc)
    },
  })
}

export function useDeleteIncomeTransaction(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/income-transactions/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['income-transactions', periodId] })
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['income-source-transaction-descriptions'] })
      invalidatePeriodAggregates(qc)
    },
  })
}
