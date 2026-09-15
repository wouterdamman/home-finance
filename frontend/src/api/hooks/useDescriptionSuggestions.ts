import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'

export interface DescriptionPreset {
  id: number
  description: string
  sortOrder: number
}

export function useCategoryDescriptionPresets(categoryId: number) {
  return useQuery<DescriptionPreset[]>({
    queryKey: ['category-description-presets', categoryId],
    queryFn: () => api.get<DescriptionPreset[]>(`/api/categories/${categoryId}/description-presets`),
    enabled: categoryId > 0,
  })
}

export function useCreateCategoryDescriptionPreset(categoryId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (description: string) => api.post(`/api/categories/${categoryId}/description-presets`, { description, sortOrder: 0 }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['category-description-presets', categoryId] }),
  })
}

export function useDeleteCategoryDescriptionPreset(categoryId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/category-description-presets/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['category-description-presets', categoryId] }),
  })
}

export function useIncomeSourceDescriptionPresets(sourceId: number) {
  return useQuery<DescriptionPreset[]>({
    queryKey: ['income-source-description-presets', sourceId],
    queryFn: () => api.get<DescriptionPreset[]>(`/api/income-sources/${sourceId}/description-presets`),
    enabled: sourceId > 0,
  })
}

export function useCreateIncomeSourceDescriptionPreset(sourceId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (description: string) => api.post(`/api/income-sources/${sourceId}/description-presets`, { description, sortOrder: 0 }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['income-source-description-presets', sourceId] }),
  })
}

export function useDeleteIncomeSourceDescriptionPreset(sourceId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.delete(`/api/income-source-description-presets/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['income-source-description-presets', sourceId] }),
  })
}

// Auto-derived from typed history (GROUP BY description), deliberately kept
// separate from the curated presets above — the transaction-entry
// Autocomplete merges the two client-side, presets pinned first.
export interface TransactionDescription {
  description: string
  lastUsed: string
  count: number
}

export function useCategoryTransactionDescriptions(categoryId: number) {
  return useQuery<TransactionDescription[]>({
    queryKey: ['category-transaction-descriptions', categoryId],
    queryFn: () => api.get<TransactionDescription[]>(`/api/categories/${categoryId}/transaction-descriptions`),
    enabled: categoryId > 0,
  })
}

export function useIncomeSourceTransactionDescriptions(sourceId: number) {
  return useQuery<TransactionDescription[]>({
    queryKey: ['income-source-transaction-descriptions', sourceId],
    queryFn: () => api.get<TransactionDescription[]>(`/api/income-sources/${sourceId}/transaction-descriptions`),
    enabled: sourceId > 0,
  })
}

// Presets first (curated, pinned), then typed history minus anything already
// covered by a preset — exact-string dedupe, per AGENTS.md.
export function mergeDescriptionSuggestions(
  presets: DescriptionPreset[] | undefined,
  history: TransactionDescription[] | undefined,
): string[] {
  const pinned = (presets ?? []).map((p) => p.description)
  const seen = new Set(pinned)
  const rest: string[] = []
  for (const h of history ?? []) {
    if (h.description === '' || seen.has(h.description)) continue
    seen.add(h.description)
    rest.push(h.description)
  }
  return [...pinned, ...rest]
}
