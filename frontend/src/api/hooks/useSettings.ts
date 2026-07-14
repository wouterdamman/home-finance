import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'

export interface Category {
  id: number
  name: string
  defaultAmountCents: number
  isItemized: boolean
  includeInTemplate: boolean
  sortOrder: number
  archivedAt?: string
}

export interface IncomeSource {
  id: number
  name: string
  defaultAmountCents: number
  includeInTemplate: boolean
  sortOrder: number
  archivedAt?: string
}

export interface Pot {
  id: number
  name: string
  kind: string
  sortOrder: number
  archivedAt?: string
}

export function useCategories() {
  return useQuery<Category[]>({
    queryKey: ['categories'],
    queryFn: () => api.get<Category[]>('/api/categories'),
  })
}

export function useCreateCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; sortOrder: number }) =>
      api.post('/api/categories', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['categories'] }),
  })
}

export function useUpdateCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean }) =>
      api.put(`/api/categories/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['categories'] }),
  })
}

export function useArchiveCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post(`/api/categories/${id}/archive`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['categories'] }),
  })
}

export function useIncomeSources() {
  return useQuery<IncomeSource[]>({
    queryKey: ['income-sources'],
    queryFn: () => api.get<IncomeSource[]>('/api/income-sources'),
  })
}

export function useCreateIncomeSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; defaultAmountCents: number; includeInTemplate: boolean; sortOrder: number }) =>
      api.post('/api/income-sources', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['income-sources'] }),
  })
}

export function useUpdateIncomeSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; name: string; defaultAmountCents: number; includeInTemplate: boolean; sortOrder: number }) =>
      api.put(`/api/income-sources/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['income-sources'] }),
  })
}

export function useArchiveIncomeSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post(`/api/income-sources/${id}/archive`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['income-sources'] }),
  })
}

export function usePots() {
  return useQuery<Pot[]>({
    queryKey: ['pots'],
    queryFn: () => api.get<Pot[]>('/api/pots'),
  })
}

export function useCreatePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; kind: string; sortOrder: number }) =>
      api.post('/api/pots', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pots'] }),
  })
}

export function useUpdatePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; name: string; kind: string; sortOrder: number }) =>
      api.put(`/api/pots/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pots'] }),
  })
}

export function useArchivePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post(`/api/pots/${id}/archive`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pots'] }),
  })
}
