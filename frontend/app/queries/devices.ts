import { defineQueryOptions } from '@pinia/colada'

/**
 * This account's signed-in devices (#85). Kept current by devices.updated (sign-ins, sign-outs,
 * push registration) and reread when the window regains focus, so last-seen times and live
 * connection status stay fresh without polling.
 */
export const devicesQuery = defineQueryOptions({
  key: ['devices'],
  query: () => apiClient().devices(),
  staleTime: 30_000,
  refetchOnWindowFocus: true,
})
