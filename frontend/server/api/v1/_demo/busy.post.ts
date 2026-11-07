import { clampLoad, startBusyLoad, stopBusyLoad } from '../../../utils/busy-load'

/**
 * Demo mode only: starts (or with `{ stop: true }` stops) the busy-channel load for this
 * account's demo session. Body: `{ members, history, rate, churn, seconds }`, clamped.
 */
export default defineEventHandler(async (event) => {
  if (!useRuntimeConfig(event).demo) throw createError({ statusCode: 404 })
  const session = await demoSession(event)
  const body = ((await readBody(event).catch(() => null)) ?? {}) as Record<string, unknown>
  if (body.stop) {
    stopBusyLoad(session)
    return { stopped: true }
  }
  const load = clampLoad(body)
  startBusyLoad(session, load)
  return load
})
