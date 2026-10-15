import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import type { Plugin, ProxyOptions } from 'vite';
import { defineConfig } from 'vitest/config';

import { PRODUCT_NAME } from './src/core/product.ts';

const hubUrl = process.env.DSX_DEV_HUB_URL ?? 'http://127.0.0.1:7300';
const outDir = fileURLToPath(new URL('../../hub/webdist/dist', import.meta.url));

/** Routes served by the hub; Vite forwards them during development. */
const hubRoutes = ['/rpc', '/api', '/files', '/healthz', '/readyz'];
const hubSocketRoutes = ['/gateway', '/lk'];

const proxy: Record<string, ProxyOptions> = {};
for (const route of hubRoutes) proxy[route] = { target: hubUrl };
for (const route of hubSocketRoutes) proxy[route] = { target: hubUrl, ws: true };

function productTitle(): Plugin {
  return {
    name: 'dislackso:product-title',
    transformIndexHtml: () => [{ tag: 'title', children: PRODUCT_NAME, injectTo: 'head' }],
  };
}

/** go:embed needs at least one file in hub/webdist/dist, even before a build. */
function keepEmbedPlaceholder(dir: string): Plugin {
  return {
    name: 'dislackso:keep-embed-placeholder',
    apply: 'build',
    closeBundle: () => {
      writeFileSync(join(dir, '.gitkeep'), '');
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), productTitle(), keepEmbedPlaceholder(outDir)],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy,
  },
  build: {
    outDir,
    emptyOutDir: true,
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test/setup.ts'],
  },
});
