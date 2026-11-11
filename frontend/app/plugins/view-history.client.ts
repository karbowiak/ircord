import { installViewHistoryEvents, removeViewHistoryEvents } from '../utils/view-history'

export default defineNuxtPlugin({
  name: 'view-history-events',
  // Vue Router is a pre plugin (-20). Its bookkeeping must follow our popstate handler.
  order: -30,
  setup(nuxtApp) {
    installViewHistoryEvents()
    nuxtApp.vueApp.onUnmount(removeViewHistoryEvents)
  },
})

if (import.meta.hot) import.meta.hot.dispose(removeViewHistoryEvents)
