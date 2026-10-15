import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import globals from 'globals';
import tseslint from 'typescript-eslint';

const TS_FILES = ['**/*.{ts,tsx,mts,cts}'];
const JS_FILES = ['**/*.{js,mjs,cjs}'];
const TEST_FILES = ['**/*.test.{ts,tsx}', '**/test/**/*.{ts,tsx}', 'e2e/**/*.ts'];

/**
 * Type-aware strict rules for every TypeScript file in the repo.
 * @param {{ tsconfigRootDir: string }} options
 */
export function typescript({ tsconfigRootDir }) {
  return [
    js.configs.recommended,
    ...tseslint.configs.strictTypeChecked.map((c) => ({ ...c, files: TS_FILES })),
    ...tseslint.configs.stylisticTypeChecked.map((c) => ({ ...c, files: TS_FILES })),
    {
      files: TS_FILES,
      languageOptions: {
        parserOptions: { projectService: true, tsconfigRootDir },
      },
      rules: {
        '@typescript-eslint/consistent-type-imports': 'error',
        '@typescript-eslint/no-import-type-side-effects': 'error',
        '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
        '@typescript-eslint/no-misused-promises': [
          'error',
          { checksVoidReturn: { attributes: false } },
        ],
        '@typescript-eslint/switch-exhaustiveness-check': 'error',
        'no-restricted-syntax': [
          'error',
          { selector: 'ExportDefaultDeclaration', message: 'Use named exports.' },
        ],
      },
    },
    {
      files: TEST_FILES,
      rules: {
        '@typescript-eslint/no-non-null-assertion': 'off',
      },
    },
    {
      files: JS_FILES,
      ...tseslint.configs.disableTypeChecked,
      languageOptions: { globals: globals.node },
    },
  ];
}

/**
 * React rules for browser code.
 * @param {{ files: string[] }} options
 */
export function react({ files }) {
  return [
    { ...reactHooks.configs.flat['recommended-latest'], files },
    { ...reactRefresh.configs.vite, files },
    { files, languageOptions: { globals: globals.browser } },
  ];
}

/**
 * Keeps framework-agnostic code free of React and UI libraries.
 * @param {{ files: string[] }} options
 */
export function frameworkAgnostic({ files }) {
  return [
    {
      files,
      rules: {
        'no-restricted-imports': [
          'error',
          {
            patterns: [
              {
                regex: '^(react|react-dom)(/.*)?$|^@tanstack/react-|^react-i18next$|^lucide-react$',
                message: 'core/* is framework-agnostic: React belongs in features/, ui/ and app/.',
              },
            ],
          },
        ],
      },
    },
  ];
}

/**
 * Config files whose tools require a default export.
 * @param {{ files: string[] }} options
 */
export function allowDefaultExport({ files }) {
  return [{ files, rules: { 'no-restricted-syntax': 'off' } }];
}
