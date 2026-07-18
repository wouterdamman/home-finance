import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Suspense, lazy } from 'react'
import { LoadingOverlay } from '@mantine/core'
import AuthGuard from './components/AuthGuard'
import AppShell from './components/AppShell'

const Login = lazy(() => import('./pages/Login'))
const YearDashboard = lazy(() => import('./pages/YearDashboard'))
const Trends = lazy(() => import('./pages/Trends'))
const MonthCompareDetail = lazy(() => import('./pages/MonthCompareDetail'))
const MonthOverview = lazy(() => import('./pages/MonthOverview'))
const MonthTransactions = lazy(() => import('./pages/MonthTransactions'))
const IncomeTransactions = lazy(() => import('./pages/IncomeTransactions'))
const Settings = lazy(() => import('./pages/Settings'))
const Pots = lazy(() => import('./pages/Pots'))
const PotDetail = lazy(() => import('./pages/PotDetail'))

const fallback = <LoadingOverlay visible />

export const router = createBrowserRouter([
  { path: '/login', element: <Suspense fallback={fallback}><Login /></Suspense> },
  {
    element: <AuthGuard><AppShell /></AuthGuard>,
    children: [
      { index: true, element: <Navigate to={`/years/${new Date().getFullYear()}`} replace /> },
      { path: '/years/:year', element: <Suspense fallback={fallback}><YearDashboard /></Suspense> },
      { path: '/trends', element: <Suspense fallback={fallback}><Trends /></Suspense> },
      { path: '/trends/months', element: <Suspense fallback={fallback}><MonthCompareDetail /></Suspense> },
      { path: '/months/:year/:month', element: <Suspense fallback={fallback}><MonthOverview /></Suspense> },
      { path: '/months/:year/:month/transactions', element: <Suspense fallback={fallback}><MonthTransactions /></Suspense> },
      { path: '/months/:year/:month/income', element: <Suspense fallback={fallback}><IncomeTransactions /></Suspense> },
      { path: '/pots', element: <Suspense fallback={fallback}><Pots /></Suspense> },
      { path: '/pots/:id', element: <Suspense fallback={fallback}><PotDetail /></Suspense> },
      { path: '/settings', element: <Suspense fallback={fallback}><Settings /></Suspense> },
    ],
  },
])
