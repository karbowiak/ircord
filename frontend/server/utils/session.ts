import { createError, type H3Event } from 'h3'
import { createSession, findSession } from './demo-store'

/** The signed-in account's demo chat state, created on first use. */
export async function demoSession(event: H3Event) {
  const account = await requireAccount(event)
  const existing = findSession(account.id)
  if (existing) return existing
  try {
    return createSession(account.id)
  } catch {
    throw createError({
      statusCode: 503,
      statusMessage: 'The demo is busy. Please try again shortly.',
    })
  }
}
export function routeError(error: unknown): never {
  throw createError({
    statusCode: error instanceof Error && /not found/.test(error.message) ? 404 : 400,
    statusMessage:
      error instanceof Error ? error.message : 'Something went wrong. Please try again.',
  })
}
