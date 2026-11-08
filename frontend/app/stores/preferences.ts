import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import {
  defaultPreferences,
  sanitizePreferences,
  type ClientPreferences,
} from '../../shared/preferences'

const key = ['preferences']
const localKey = 'ircord.preferences'

/** Event display and network icons, synced to the account. */
export const usePreferencesStore = defineStore('preferences', () => {
  const queryCache = useQueryCache()
  const query = useQuery({
    key,
    query: () => appClient.preferences(),
    placeholderData: () => ({
      preferences: readLocal(localKey, sanitizePreferences),
      updatedAt: null,
    }),
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  })
  const preferences = computed<ClientPreferences>(
    () => query.data.value?.preferences ?? defaultPreferences,
  )
  watch(preferences, (value) => writeLocal(localKey, value))

  const queue = createLatestQueue(
    (value: ClientPreferences) =>
      appClient.savePreferences(value),
    {
      delay: 400,
      onSaved: (_value, saved) => {
        queryCache.cancelQueries({ key })
        queryCache.setQueryData(key, saved)
      },
      onError: () =>
        useChatStore().notify('Couldn’t save your preferences. They apply on this device.'),
    },
  )
  function update(changes: Partial<ClientPreferences>) {
    const next = sanitizePreferences({ ...preferences.value, ...changes })
    queryCache.setQueryData(key, {
      preferences: next,
      updatedAt: query.data.value?.updatedAt ?? null,
    })
    queue.push(next)
  }

  function setIcon(networkId: string, dataUrl: string | null) {
    const networkIcons = { ...preferences.value.networkIcons }
    if (dataUrl) networkIcons[networkId] = dataUrl
    else delete networkIcons[networkId]
    update({ networkIcons })
  }

  return { query, preferences, update, setIcon }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(usePreferencesStore, import.meta.hot))
