import type { components } from '../../shared/generated/api'
import { createPeopleCache } from '../../shared/utils/people-lookup'

/** Shared only within the current browser session; auth changes clear it immediately. */
export const peopleLookupCache = createPeopleCache<components['schemas']['Person']>()
