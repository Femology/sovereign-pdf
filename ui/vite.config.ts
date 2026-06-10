import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Forward any incoming browser traffic starting with /api/ directly to local Go executable
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false, // In local dev we do not require HTTPS
      },
    },
  },
});
