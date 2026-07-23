import { useQuery } from '@tanstack/react-query'
import { api } from '../client'

interface DescriptionSuggestion {
  description: string
  lastUsed: string
  count: number
}

export function useCategoryDescriptionSuggestions(categoryId: number) {
  return useQuery<DescriptionSuggestion[]>({
    queryKey: ['category-descriptions', categoryId],
    queryFn: () => api.get<DescriptionSuggestion[]>(`/api/categories/${categoryId}/transaction-descriptions?limit=15`),
    enabled: categoryId > 0,
  })
}

export function useIncomeSourceDescriptionSuggestions(sourceId: number) {
  return useQuery<DescriptionSuggestion[]>({
    queryKey: ['income-source-descriptions', sourceId],
    queryFn: () => api.get<DescriptionSuggestion[]>(`/api/income-sources/${sourceId}/transaction-descriptions?limit=15`),
    enabled: sourceId > 0,
  })
}
