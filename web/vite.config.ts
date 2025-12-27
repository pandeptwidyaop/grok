import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173, // Vite default port (avoid conflict with API)
    proxy: {
      '/api': {
        target: 'http://localhost:4040', // API server port from server.yaml
        changeOrigin: true,
        ws: true, // Enable WebSocket proxying for SSE
      },
    },
  },
  build: {
    outDir: '../internal/server/web/dist',
    emptyOutDir: true,
  },
})
