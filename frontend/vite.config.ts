import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg', 'apple-touch-icon.png'],
      manifest: {
        name: 'Home Finance',
        short_name: 'Finance',
        description: 'Huishoudboekje: budgetten, transacties en spaarpotjes',
        theme_color: '#0ca678',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: 'maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,ico}'],
        // /auth/login and /auth/callback are server-handled OIDC redirects, and
        // /api/docs + /api/openapi.yaml are server-rendered pages opened via
        // window.open (a full-page navigation, not a fetch/XHR) — none are SPA
        // routes. Without this, Workbox's default NavigationRoute serves the cached
        // index.html for every navigation (matched against pathname+search) before
        // it ever reaches the runtimeCaching rule below, so these paths 404 in
        // React Router instead of hitting the backend — same failure mode that
        // broke login for anyone whose browser already had the service worker
        // installed from a prior visit.
        navigateFallbackDenylist: [/^\/auth\//, /^\/api\//],
        runtimeCaching: [
          {
            urlPattern: /^\/api\//,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'api-cache',
              networkTimeoutSeconds: 5,
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
  },
})
