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

export function useYears() {
  return useQuery<number[]>({
    queryKey: ['years'],
    queryFn: () => api.get<number[]>('/api/years'),
  })
}

export function useCreateYear() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (year: number) => api.post('/api/years', { year }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['years'] }),
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

// Closing/reopening a December period can create or remove a carryover
// entry in next January's period, so the following year's summary needs
// invalidating too (not just the current one).
function yearsTouchedByPeriodMutation(year: number, month: number): number[] {
  return month === 12 ? [year, year + 1] : [year]
}

export function useClosePeriod(periodId: number, year: number, month: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post(`/api/periods/${periodId}/close`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId] })
      for (const y of yearsTouchedByPeriodMutation(year, month)) {
        qc.invalidateQueries({ queryKey: ['year-summary', y] })
      }
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

export function useCreateBudgetLine(periodId: number, year: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { categoryId: number; amountCents: number; tracksTransactions: boolean; sortOrder: number }) =>
      api.post(`/api/periods/${periodId}/budget-lines`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['year-summary', year] })
    },
  })
}

export function useUpdateBudgetLine(periodId: number, year: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; label: string | null; amountCents: number; tracksTransactions: boolean; sortOrder: number }) =>
      api.put(`/api/budget-lines/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      qc.invalidateQueries({ queryKey: ['year-summary', year] })
    },
  })
}

export function useLockYear(year: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (password: string) => api.post(`/api/years/${year}/lock`, { password }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['year-summary', year] }),
  })
}

export function useUnlockYear(year: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (password: string) => api.post(`/api/years/${year}/unlock`, { password }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['year-summary', year] }),
  })
}

export function useReopenPeriod(periodId: number, year: number, month: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post(`/api/periods/${periodId}/reopen`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId] })
      for (const y of yearsTouchedByPeriodMutation(year, month)) {
        qc.invalidateQueries({ queryKey: ['year-summary', y] })
      }
      qc.invalidateQueries({ queryKey: ['pots'] })
    },
  })
}
