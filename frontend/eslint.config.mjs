import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt({
  name: 'ircord/generated',
  ignores: [
    'shared/generated/**',
    'public/data/**',
    'server/assets/**',
    '.nuxt/**',
    '.output/**',
    'node_modules/**',
  ],
})
