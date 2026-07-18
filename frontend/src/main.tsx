import React from 'react'
import ReactDOM from 'react-dom/client'
import { MantineProvider, createTheme } from '@mantine/core'
import { Notifications, notifications } from '@mantine/notifications'
import { ModalsProvider } from '@mantine/modals'
import { QueryClient, QueryClientProvider, MutationCache } from '@tanstack/react-query'
import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'
import '@mantine/charts/styles.css'
import '@mantine/dates/styles.css'
import 'react-swipeable-list/dist/styles.css'
import i18n from './i18n/index'
import App from './App'
import { getErrorMessage } from './api/client'
import { ChartPaletteProvider } from './contexts/ChartPaletteContext'

// Safety net: any mutation without its own onError still surfaces a
// notification instead of failing silently (button just stops loading).
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
  mutationCache: new MutationCache({
    onError: (error) => {
      notifications.show({ color: 'red', message: getErrorMessage(error, i18n.t('common.error')) })
    },
  }),
})

const fontFamily = '-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif'

const theme = createTheme({
  primaryColor: 'teal',
  defaultRadius: 'md',
  fontFamily,
  headings: { fontFamily, fontWeight: '700' },
})

const rootEl = document.getElementById('root')
if (!rootEl) throw new Error('root element not found')

ReactDOM.createRoot(rootEl).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <MantineProvider theme={theme} defaultColorScheme="auto">
        <ModalsProvider>
          <Notifications />
          <ChartPaletteProvider>
            <App />
          </ChartPaletteProvider>
        </ModalsProvider>
      </MantineProvider>
    </QueryClientProvider>
  </React.StrictMode>,
)
