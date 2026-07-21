/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { resolve } from 'node:path';
import { readFileSync } from 'node:fs';

const pkgVersion = JSON.parse(
  readFileSync(resolve(__dirname, 'package.json'), 'utf8'),
).version as string;

// aw frontend lives at frontend/ (index.html here); Wails reads the build from
// frontend/dist. Keep root at the frontend dir so Wails wiring stays intact.
export default defineConfig({
  plugins: [tailwindcss(), react()],
  define: {
    __APP_VERSION__: JSON.stringify(pkgVersion),
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@ui': resolve(__dirname, 'src/components/ui'),
      '@patterns': resolve(__dirname, 'src/components/patterns'),
      '@services': resolve(__dirname, 'src/services'),
      '@modules': resolve(__dirname, 'src/modules'),
      '@stores': resolve(__dirname, 'src/stores'),
      '@hooks': resolve(__dirname, 'src/hooks'),
      '@wails': resolve(__dirname, 'wailsjs'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
