import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import type { Period, MonthOverview, YearSummary } from '../types'

export function usePeriods(year: number) {
  return useQuery<Period[]>({
    queryKey: ['periods', year],
    queryFn: () => api.get<Period[]>(`/api/periods?year=${year}`),
  })
}

export function useYearSummary(year: number) {
  return useQuery<YearSummary>({
    queryKey: ['year-summary', year],
    queryFn: () => api.get<YearSummary>(`/api/years/${year}/summary`),
  })
}

export function useMonthOverview(periodId: number | undefined) {
  return useQuery<MonthOverview>({
    queryKey: ['period', periodId, 'overview'],
    queryFn: () => api.get<MonthOverview>(`/api/periods/${periodId}/overview`),
    enabled: periodId != null,
  })
}

export function useCreatePeriod() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { year: number; month: number; copyFromPeriodId?: number }) =>
      api.post<Period>('/api/periods', body),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ['periods', vars.year] })
      qc.invalidateQueries({ queryKey: ['year-summary', vars.year] })
    },
  })
}

export function useClosePeriod(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post(`/api/periods/${periodId}/close`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId] })
      qc.invalidateQueries({ queryKey: ['year-summary'] })
      qc.invalidateQueries({ queryKey: ['pots'] })
    },
  })
}

export function useDeletePeriod(periodId: number, year: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (password: string) =>
      api.delete_body(`/api/periods/${periodId}`, { password }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['year-summary', year] })
      qc.invalidateQueries({ queryKey: ['periods', year] })
    },
  })
}

export function useUpdateBudgetLine(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; label: string; amountCents: number; tracksTransactions: boolean; sortOrder: number }) =>
      api.put(`/api/budget-lines/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] }),
  })
}

export function useReopenPeriod(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post(`/api/periods/${periodId}/reopen`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId] })
      qc.invalidateQueries({ queryKey: ['year-summary'] })
      qc.invalidateQueries({ queryKey: ['pots'] })
    },
  })
}
