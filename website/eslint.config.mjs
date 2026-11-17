import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt(
  {
    name: 'ircord-website/generated',
    ignores: ['.nuxt/**', '.output/**', 'node_modules/**', 'public/**'],
  },
  {
    name: 'ircord-website/rules',
    rules: {
      // Prettier owns formatting and writes void elements as `<img />`; the rule would fight it.
      'vue/html-self-closing': [
        'warn',
        { html: { void: 'any', normal: 'always', component: 'always' } },
      ],
    },
  },
)
