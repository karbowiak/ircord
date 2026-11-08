import { defineQueryOptions } from '@pinia/colada'

/**
 * Whether the signed-in account is a server owner (#165). The backend checks ownership on every
 * Administration request, so this only decides what to show; it's reread on focus and when a
 * request is refused, so a revoked owner loses the entry quickly.
 */
export const administrationAccessQuery = defineQueryOptions({
  key: ['administration', 'access'],
  query: () => apiClient().administrationAccess(),
  staleTime: 60_000,
  refetchOnWindowFocus: true,
})

export const administrationStatusQuery = defineQueryOptions({
  key: ['administration', 'status'],
  query: () => apiClient().administrationStatus(),
  staleTime: 10_000,
  refetchOnWindowFocus: true,
})

export const administrationAuditQuery = defineQueryOptions({
  key: ['administration', 'audit'],
  query: () => apiClient().administrationAudit(30),
  staleTime: 10_000,
})
