import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { notifications } from '@mantine/notifications'
import { api } from '../client'
import i18n from '../../i18n/index'
import type { PotBalance, PotLedgerEntry } from '../types'

function showSaved() {
  notifications.show({ color: 'green', message: i18n.t('common.saved') })
}

export interface Category {
  id: number
  name: string
  defaultAmountCents: number
  isItemized: boolean
  includeInTemplate: boolean
  sortOrder: number
  archivedAt?: string
  parentId?: number
  autofillActual?: boolean
}

export interface CategoryAlias {
  id: number
  aliasName: string
  parentCategoryId: number
}

export interface IncomeSource {
  id: number
  name: string
  defaultAmountCents: number
  isItemized: boolean
  includeInTemplate: boolean
  sortOrder: number
  archivedAt?: string
  autofillActual?: boolean
}

export interface Pot {
  id: number
  name: string
  kind: string
  sortOrder: number
  targetCents?: number
  targetDate?: string
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
    mutationFn: (body: { name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; sortOrder: number; parentId?: number | null; autofillActual?: boolean }) =>
      api.post('/api/categories', body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['categories'] }); showSaved() },
  })
}

export function useUpdateCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; parentId?: number | null; autofillActual?: boolean }) =>
      api.put(`/api/categories/${id}`, body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['categories'] }); showSaved() },
  })
}

export function useCategoryAliases() {
  return useQuery<CategoryAlias[]>({
    queryKey: ['category-aliases'],
    queryFn: () => api.get<CategoryAlias[]>('/api/category-aliases'),
  })
}

export function useCreateCategoryAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { aliasName: string; parentCategoryId: number }) =>
      api.post('/api/category-aliases', body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['category-aliases'] }); showSaved() },
  })
}

export function useDeleteCategoryAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/category-aliases/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['category-aliases'] }),
  })
}

export function useArchiveCategory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post(`/api/categories/${id}/archive`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['categories'] }); showSaved() },
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
    mutationFn: (body: { name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; sortOrder: number; autofillActual?: boolean }) =>
      api.post('/api/income-sources', body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['income-sources'] }); showSaved() },
  })
}

export function useUpdateIncomeSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; name: string; defaultAmountCents: number; isItemized: boolean; includeInTemplate: boolean; sortOrder: number; autofillActual?: boolean }) =>
      api.put(`/api/income-sources/${id}`, body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['income-sources'] }); showSaved() },
  })
}

export function useArchiveIncomeSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post(`/api/income-sources/${id}/archive`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['income-sources'] }); showSaved() },
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
    mutationFn: (body: { name: string; kind: string; sortOrder: number; targetCents?: number | null; targetDate?: string | null }) =>
      api.post('/api/pots', body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['pots'] }); showSaved() },
  })
}

export function useUpdatePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number; name: string; kind: string; sortOrder: number; targetCents?: number | null; targetDate?: string | null }) =>
      api.put(`/api/pots/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pots'] })
      qc.invalidateQueries({ queryKey: ['pot-balances'] })
      showSaved()
    },
  })
}

export function useArchivePot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post(`/api/pots/${id}/archive`),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['pots'] }); showSaved() },
  })
}

export function usePotBalances() {
  return useQuery<PotBalance[]>({
    queryKey: ['pot-balances'],
    queryFn: () => api.get<PotBalance[]>('/api/pots/balances'),
  })
}

export function usePotLedger(potId: number | undefined) {
  return useQuery<PotLedgerEntry[]>({
    queryKey: ['pot-ledger', potId],
    queryFn: () => api.get<PotLedgerEntry[]>(`/api/pots/${potId}/ledger`),
    enabled: potId != null,
  })
}

export function useCreatePotEntry(potId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { entryType: string; amountCents: number; description: string; entryDate?: string }) =>
      api.post(`/api/pots/${potId}/entries`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pot-ledger', potId] })
      qc.invalidateQueries({ queryKey: ['pot-balances'] })
      qc.invalidateQueries({ queryKey: ['year-summary'] })
    },
  })
}

export function useUpdatePotEntry(potId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, amountCents }: { id: number; amountCents: number }) =>
      api.patch(`/api/pot-entries/${id}`, { amountCents }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pot-ledger', potId] })
      qc.invalidateQueries({ queryKey: ['pot-balances'] })
      qc.invalidateQueries({ queryKey: ['year-summary'] })
    },
  })
}

export function useDeletePotEntry(potId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/pot-entries/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pot-ledger', potId] })
      qc.invalidateQueries({ queryKey: ['pot-balances'] })
      qc.invalidateQueries({ queryKey: ['year-summary'] })
    },
  })
}
