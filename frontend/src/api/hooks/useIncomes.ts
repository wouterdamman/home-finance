import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import type { IncomeEntry } from '../types'

export function useCreateIncome(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { sourceId?: number; label?: string; amountCents: number; notes: string; sortOrder: number }) =>
      api.post<IncomeEntry>(`/api/periods/${periodId}/incomes`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] }),
  })
}

export function useUpdateIncome(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; label?: string; amountCents: number; notes: string; sortOrder: number }) =>
      api.put(`/api/incomes/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] }),
  })
}

export function useDeleteIncome(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/incomes/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] }),
  })
}

export function useIncomeSources() {
  return useQuery({
    queryKey: ['income-sources'],
    queryFn: () => api.get<{ id: number; name: string; defaultAmountCents: number; sortOrder: number }[]>('/api/income-sources'),
  })
}
