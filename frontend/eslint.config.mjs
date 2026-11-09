import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt(
  {
    name: 'ircord/generated',
    ignores: [
      'shared/generated/**',
      'public/data/**',
      'server/assets/**',
      '.nuxt/**',
      '.output/**',
      'node_modules/**',
    ],
  },
  {
    name: 'ircord/rules',
    rules: {
      // Optional TypeScript props are deliberately `undefined` when unset: components tell
      // "not provided" from an empty value, and forwarded props must not receive a default.
      'vue/require-default-prop': 'off',
      // Sparse records (per-conversation drafts, read markers, mock state) are serialized
      // contracts: a missing key means "none" and `undefined`/empty values must stay distinct.
      '@typescript-eslint/no-dynamic-delete': 'off',
      // Prettier owns formatting and writes void elements as `<img />`; the rule would fight it.
      'vue/html-self-closing': [
        'warn',
        { html: { void: 'any', normal: 'always', component: 'always' } },
      ],
    },
  },
)
