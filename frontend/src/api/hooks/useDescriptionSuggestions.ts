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
