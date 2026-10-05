import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist', 'test-results', 'playwright-report', 'blob-report']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
  },
  {
    // The SDK's isLoading flips whenever the client touches the token, which
    // switched every query off and on and refetched it. App code reads the
    // hook through @hooks/useStableAsgardeo instead (see that file).
    files: ['src/**/*.{ts,tsx}'],
    ignores: [
      'src/hooks/useStableAsgardeo.ts',
      'src/**/__tests__/**',
      'src/**/*.test.{ts,tsx}',
      'src/vitest.setup.ts',
    ],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [
            {
              name: '@asgardeo/react',
              importNames: ['useAsgardeo'],
              message:
                "Import useAsgardeo from '@hooks/useStableAsgardeo': the SDK's isLoading flag flips during token activity and causes request loops.",
            },
          ],
        },
      ],
    },
  },
])
