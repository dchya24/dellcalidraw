import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
    plugins: [react(), tailwindcss()],
    server: {
      port: Number(env.VITE_PORT) || 5173,
      proxy: {
        '/api': {
          target: env.VITE_API_URL || 'http://localhost:8081',
          changeOrigin: true,
          // Disable buffering for SSE streaming
          configure: (proxy) => {
            proxy.on('proxyRes', (proxyRes) => {
              // Disable gzip compression for SSE
              delete proxyRes.headers['content-encoding']
            })
          },
        },
      },
    },
    build: {
      // Extend timeout for large builds
      sourcemap: false,
    },
    // Vitest config piggybacks on vite.config so plugins are reused.
    test: {
      globals: true,
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      // Skip mermaid/excalidraw heavy bundles — they're integration
      // territory and need real DOM/canvas. Unit tests focus on the
      // service layer, hooks, and small components.
      exclude: ['node_modules', 'dist', '.git', '.idea', 'e2e/**'],
      css: false,
    },
  }
})
