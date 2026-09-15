import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import { invalidatePeriodAggregates } from '../../lib/queryInvalidation'

export function useReplaceSplits(periodId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (splits: { potId: number; percentage: string }[]) =>
      api.put(`/api/periods/${periodId}/splits`, { splits }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['period', periodId, 'overview'] })
      invalidatePeriodAggregates(qc)
    },
  })
}
