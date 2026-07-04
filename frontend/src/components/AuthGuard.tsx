import { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { LoadingOverlay } from '@mantine/core'
import { useMe } from '../api/hooks/useMe'

export default function AuthGuard({ children }: { children: ReactNode }) {
  const { data: user, isLoading } = useMe()
  const location = useLocation()

  if (isLoading) return <LoadingOverlay visible />
  if (!user) return <Navigate to="/login" state={{ from: location }} replace />
  return <>{children}</>
}
