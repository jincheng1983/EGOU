import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import wails from '@wailsio/runtime/plugins/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), wails('./bindings')],
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // H5：拆分大依赖到独立 chunk，减小主包体积，提升首屏加载速度
    // 注意：rolldown 不支持 manualChunks 对象形式，用函数形式按 id 归类
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/monaco-editor') || id.includes('node_modules\\monaco-editor')) return 'monaco'
          if (id.includes('node_modules/naive-ui') || id.includes('node_modules\\naive-ui')) return 'naive'
          if (id.includes('node_modules/@vue') || id.includes('node_modules\\@vue') || id.includes('node_modules/vue') || id.includes('node_modules\\vue')) return 'vendor'
        }
      }
    }
  }
})
