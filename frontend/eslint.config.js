import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  { ignores: ['dist'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // react-hooks 7 added React Compiler diagnostics to the historical
      // recommended preset. OmniLLM-Studio has not enabled the compiler and
      // intentionally uses effect-driven store synchronization and mutable
      // media refs throughout its editors. Keep the established hooks safety
      // rules while the compiler-specific migration is handled separately.
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/refs': 'off',
      'react-hooks/purity': 'off',
      'react-hooks/preserve-manual-memoization': 'off',
      // ESLint 9.39 enabled this rule in the recommended baseline. Existing
      // editor control-flow uses defensive reassignments that are harmless and
      // should be simplified in focused refactors rather than blocking builds.
      'no-useless-assignment': 'off',
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
    },
  },
)
