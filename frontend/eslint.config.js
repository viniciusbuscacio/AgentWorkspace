import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: ['dist', 'wailsjs', 'node_modules'],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      // aw loads data from Wails services in effects; this rule is too broad
      // for async bootstrapping flows and creates noise without catching bugs.
      'react-hooks/set-state-in-effect': 'off',
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      // Fence: every Save button must go through SaveCancelActions
      // (Decision 6 of notifications-save-cancel-spec). The required-props
      // enforcement is the primary gate; this rule is the regression fence.
      // Use `// eslint-disable-next-line no-restricted-syntax` inside
      // SaveCancelActions.tsx itself to allow the authorised site.
      // The regex matches JSXText that is exactly "Save" or starts with
      // "Saving" — it does NOT match composite labels like "Save & activate".
      'no-restricted-syntax': [
        'error',
        {
          selector:
            'JSXElement[openingElement.name.name="Button"] > JSXText[value=/^\\s*Save\\s*$|^\\s*Saving/]',
          message:
            'Raw <Button> with text "Save" or "Saving…" is not allowed. Use <SaveCancelActions> from @patterns/SaveCancelActions instead.',
        },
      ],
      // Fence (Golden rule #2): only services/*.service.ts may import the Wails
      // app bindings (@wails/go/main/*) and runtime (@wails/runtime*). The rest
      // of the frontend talks to Go through those services. Type-only model
      // imports (@wails/go/models) stay allowed everywhere — that is the
      // sanctioned way to avoid redefining Go types. The override below exempts
      // src/services/**. This is the machine-enforced counterpart to the
      // backend's internal/architecture boundary tests.
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['@wails/go/main', '@wails/go/main/*', '@wails/runtime', '@wails/runtime/*'],
              message:
                'Wails bindings/runtime must be wrapped in a service. Import them only from src/services/*.service.ts (type-only @wails/go/models is allowed anywhere).',
            },
          ],
        },
      ],
    },
  },
  {
    // The service layer is the single sanctioned home for raw Wails bindings.
    files: ['src/services/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': 'off',
    },
  },
  {
    files: ['scripts/**/*.mjs'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.node,
    },
  },
);
