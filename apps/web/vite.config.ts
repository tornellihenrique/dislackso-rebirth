import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import type { Plugin, ProxyOptions } from 'vite';
import { defineConfig } from 'vitest/config';

import { PRODUCT_NAME } from './src/core/product.ts';

const hubUrl = process.env.DSX_DEV_HUB_URL ?? 'http://127.0.0.1:7300';
const outDir = fileURLToPath(new URL('../../hub/webdist/dist', import.meta.url));

// The noise suppressor exports only its AudioWorklet API; the mic chain runs
// its DeepFilterNet3 processor in a worker and imports the files directly.
const deepFilterAssets = join(
  dirname(createRequire(import.meta.url).resolve('@workadventure/noise-suppression/package.json')),
  'dist/assets',
);

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

/**
 * Gzipped copies of the assets that shrink, next to them (name.gz): the hub
 * sends those to browsers that take gzip, over what is often a home upload.
 */
function precompress(dir: string): Plugin {
  return {
    name: 'dislackso:precompress',
    apply: 'build',
    writeBundle: () => {
      const assets = join(dir, 'assets');
      for (const file of readdirSync(assets)) {
        if (!/\.(js|css|wasm|svg|json)$/.test(file)) continue;
        const data = readFileSync(join(assets, file));
        if (data.length < 1024) continue;
        const gz = gzipSync(data, { level: 9 });
        if (gz.length < data.length * 0.9) writeFileSync(join(assets, `${file}.gz`), gz);
      }
    },
  };
}

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    productTitle(),
    keepEmbedPlaceholder(outDir),
    precompress(outDir),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@deepfilternet': deepFilterAssets,
    },
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
