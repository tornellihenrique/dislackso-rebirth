import { resolve } from 'node:path';

import { defineConfig } from 'electron-vite';

export default defineConfig({
  main: {
    build: {
      // Workspace packages (TypeScript source) and the updater go into the
      // bundle. The keyboard hook is a native addon, so it stays a module of
      // its own, the only one the package carries in node_modules.
      externalizeDeps: false,
      rollupOptions: {
        input: { index: resolve('src/main/index.ts') },
        external: ['uiohook-napi'],
      },
    },
  },
  preload: {
    build: {
      // Sandboxed preloads load only CommonJS, and only Electron's own modules:
      // everything else has to be in the bundle.
      externalizeDeps: false,
      isolatedEntries: true,
      rollupOptions: {
        input: {
          instance: resolve('src/preload/instance.ts'),
          launcher: resolve('src/preload/launcher.ts'),
          overlay: resolve('src/preload/overlay.ts'),
        },
        output: { format: 'cjs', entryFileNames: '[name].cjs' },
      },
    },
  },
  renderer: {
    root: resolve('src/renderer'),
    build: {
      rollupOptions: {
        input: {
          launcher: resolve('src/renderer/launcher/index.html'),
          overlay: resolve('src/renderer/overlay/index.html'),
        },
      },
    },
  },
});
