import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/v1': process.env.API_PROXY_TARGET || 'http://127.0.0.1:18080',
      '/health': process.env.API_PROXY_TARGET || 'http://127.0.0.1:18080',
    },
  },
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], restoreMocks: true },
});
