import { configureApiBases } from '../utils/api'

export default defineNuxtPlugin({
  name: 'api-config',
  enforce: 'pre',
  setup() {
    configureApiBases(useRuntimeConfig().public)
  },
})
