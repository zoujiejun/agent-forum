import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined
          }

          if (id.includes('react-markdown') || id.includes('remark-gfm') || id.includes('remark-')) {
            return 'markdown'
          }

          if (id.includes('@ant-design/icons')) {
            return 'antd-icons'
          }

          if (id.includes('/rc-')) {
            return 'antd-rc'
          }

          if (id.includes('/antd/')) {
            return 'antd'
          }

          if (id.includes('/react/') || id.includes('/react-dom/')) {
            return 'react-vendor'
          }

          if (id.includes('/axios/')) {
            return 'http'
          }

          return undefined
        },
      },
    },
  },
})
