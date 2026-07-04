import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../client'
import type { Pot, PotBalance, PotLedgerEntry } from '../types'

export function usePotBalances() {
  return useQuery<PotBalance[]>({
    queryKey: ['pots', 'balances'],
    queryFn: () => api.get<PotBalance[]>('/api/pots/balances'),
  })
}

export function usePots() {
  return useQuery<Pot[]>({
    queryKey: ['pots'],
    queryFn: () => api.get<Pot[]>('/api/pots'),
  })
}

export function usePotLedger(potId: number) {
  return useQuery<PotLedgerEntry[]>({
    queryKey: ['pots', potId, 'ledger'],
    queryFn: () => api.get<PotLedgerEntry[]>(`/api/pots/${potId}/ledger`),
  })
}

export function useAddPotEntry(potId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { entryType: string; amountCents: number; description: string; entryDate: string }) =>
      api.post(`/api/pots/${potId}/entries`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pots'] })
    },
  })
}
