import type { components } from '../../../../../../shared/generated/api'

/** Update the demo session's desired notification settings. */
export default defineEventHandler(async (event) => {
  const input = await readBody<components['schemas']['DevicePreferences']>(event)
  return saveDevicePreferences(event, getRouterParam(event, 'id') ?? '', input)
})
