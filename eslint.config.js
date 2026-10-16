import { allowDefaultExport, frameworkAgnostic, react, typescript } from '@dislackso/config/eslint';
import { defineConfig, globalIgnores } from 'eslint/config';

export default defineConfig([
  globalIgnores([
    '**/node_modules/',
    '**/dist/',
    '.tools/',
    'hub/',
    'packages/contracts/src/gen/',
    'e2e/test-results/',
    'e2e/playwright-report/',
  ]),
  typescript({ tsconfigRootDir: import.meta.dirname }),
  react({ files: ['apps/web/src/**/*.{ts,tsx}'] }),
  frameworkAgnostic({ files: ['apps/web/src/core/**/*.ts'] }),
  allowDefaultExport({
    files: [
      '**/vite.config.ts',
      '**/playwright.config.ts',
      '**/playwright.bench.config.ts',
      'prettier.config.js',
      'eslint.config.js',
    ],
  }),
]);
