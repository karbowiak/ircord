import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import {
  defaultPreferences,
  sanitizePreferences,
  type ClientPreferences,
} from '../../shared/preferences'

const key = ['preferences']
const localKey = 'ircord.preferences'

/** Event display, embed loading and network icons, synced to the account. */
export const usePreferencesStore = defineStore('preferences', () => {
  const { $i18n } = useNuxtApp()
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
  // Local placeholders must not load third-party frames before the account's
  // actual privacy choice arrives. A retained server snapshot remains usable.
  const ready = computed(() => !query.isPlaceholderData.value && !!query.data.value)
  watch(preferences, (value) => writeLocal(localKey, value))

  /** Where the newest change is: waiting or being sent, stored on the server, or refused. */
  const saveState = ref<'idle' | 'saving' | 'saved' | 'failed'>('idle')
  const queue = createLatestQueue((value: ClientPreferences) => appClient.savePreferences(value), {
    delay: 400,
    onSaved: (_value, saved) => {
      queryCache.cancelQueries({ key })
      queryCache.setQueryData(key, saved)
      saveState.value = 'saved'
    },
    onError: () => {
      saveState.value = 'failed'
      useChatStore().notify($i18n.t('workspaceUi.stores.savePreferencesFailed'))
    },
  })
  function update(changes: Partial<ClientPreferences>) {
    const next = sanitizePreferences({ ...preferences.value, ...changes })
    queryCache.setQueryData(key, {
      preferences: next,
      updatedAt: query.data.value?.updatedAt ?? null,
    })
    saveState.value = 'saving'
    queue.push(next)
  }

  function setIcon(networkId: string, dataUrl: string | null) {
    const networkIcons = { ...preferences.value.networkIcons }
    if (dataUrl) networkIcons[networkId] = dataUrl
    else delete networkIcons[networkId]
    update({ networkIcons })
  }

  return { query, preferences, ready, update, setIcon, saveState }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(usePreferencesStore, import.meta.hot))
