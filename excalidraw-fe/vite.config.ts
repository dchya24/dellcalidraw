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
          target: 'http://localhost:8080',
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
  }
})
