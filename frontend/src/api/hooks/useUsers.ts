import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { notifications } from '@mantine/notifications'
import { api } from '../client'
import i18n from '../../i18n/index'
import type { User } from '../types'

export function useUpdateMe() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (displayName: string) => api.patch<void>('/api/me', { displayName }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['me'] })
      notifications.show({ color: 'green', message: i18n.t('common.saved') })
    },
  })
}

export function useUploadAvatar() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData()
      form.append('file', file)
      return api.postForm<{ avatarUrl: string }>('/api/me/avatar', form)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['me'] })
      notifications.show({ color: 'green', message: i18n.t('common.saved') })
    },
  })
}

export function useUsers() {
  return useQuery<User[]>({
    queryKey: ['users'],
    queryFn: () => api.get<User[]>('/api/users'),
  })
}

export function useUpdateUserRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, role }: { id: number; role: 'admin' | 'user' }) =>
      api.patch<void>(`/api/users/${id}/role`, { role }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] })
      notifications.show({ color: 'green', message: i18n.t('common.saved') })
    },
  })
}
