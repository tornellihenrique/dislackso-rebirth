/** @type {import('prettier').Config} */
export default {
  printWidth: 100,
  singleQuote: true,
  trailingComma: 'all',
  plugins: ['prettier-plugin-tailwindcss'],
  overrides: [
    {
      files: 'apps/web/**',
      options: { tailwindStylesheet: './apps/web/src/styles/index.css' },
    },
  ],
};
