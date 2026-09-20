import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

const r = (p: string) => fileURLToPath(new URL(p, import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': r('./src'),
      '@/app': r('./src/app'),
      '@/pages': r('./src/pages'),
      '@/widgets': r('./src/widgets'),
      '@/features': r('./src/features'),
      '@/entities': r('./src/entities'),
      '@/shared': r('./src/shared'),
    },
  },
  server: {
    port: 5173,
    // The cookies are HttpOnly and same-origin, so the dev server has to
    // carry /api itself rather than the app talking to another origin.
    proxy: {
      '/api': {
        target: process.env.BACKEND_ORIGIN ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
});
