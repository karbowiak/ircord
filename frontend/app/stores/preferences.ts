import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import { sanitizePreferences, type ClientPreferences } from '../../shared/preferences'

const key = ['preferences']

/** Event display, embed loading and network icons, synced to the account. */
export const usePreferencesStore = defineStore('preferences', () => {
  const { $i18n } = useNuxtApp()
  const queryCache = useQueryCache()
  // Keep late query completions bound to the account that created this store.
  const localKey = `ircord.preferences.${useAuthStore().account?.id ?? 'signed-out'}`
  const local = readLocal(localKey, sanitizePreferences)
  const query = useQuery({
    key,
    query: () => appClient.preferences(),
    placeholderData: () => ({
      preferences: local,
      updatedAt: null,
    }),
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  })
  // Keep a local edit in force while a refetch or an older save is in flight.
  const pending = ref<ClientPreferences>()
  const preferences = computed<ClientPreferences>(() =>
    sanitizePreferences(pending.value ?? (query.data.value ? query.data.value.preferences : local)),
  )
  // Local placeholders must not load third-party frames before the account's
  // actual privacy choice arrives. A retained server snapshot remains usable.
  const ready = computed(() => !query.isPlaceholderData.value && !!query.data.value)
  watch(preferences, (value) => writeLocal(localKey, value))

  /** Where the newest change is: waiting or being sent, stored on the server, or refused. */
  const saveState = ref<'idle' | 'saving' | 'saved' | 'failed'>('idle')
  const revision = ref(0)
  const remoteDuringSave = ref(false)
  const remoteRefreshPending = ref(false)
  let remoteRevision = 0
  let readGeneration = 0
  let revisionAtSend = 0
  const queue = createLatestQueue(
    (value: ClientPreferences) => {
      revisionAtSend = remoteRevision
      return appClient.savePreferences(value)
    },
    {
      delay: 400,
      onSaved: (_value, saved) => {
        queryCache.cancelQueries({ key })
        // A private update can be newer than this request's response.
        queryCache.setQueryData(
          key,
          revisionAtSend === remoteRevision
            ? saved
            : {
                preferences: sanitizePreferences({ ...pending.value, nsfwNetworks: {} }),
                updatedAt: null,
              },
        )
        pending.value = undefined
        remoteDuringSave.value = false
        saveState.value = 'saved'
        if (revisionAtSend !== remoteRevision) refetchPrivate()
        else {
          readGeneration++
          remoteRefreshPending.value = false
        }
      },
      onError: () => {
        // A failed optimistic choice must not mask later updates from another device.
        // Keep NSFW content hidden until the server's preference can be read again.
        const safe = sanitizePreferences({ ...pending.value, nsfwNetworks: {} })
        queryCache.cancelQueries({ key })
        queryCache.setQueryData(key, {
          preferences: safe,
          updatedAt: query.data.value?.updatedAt ?? null,
        })
        pending.value = undefined
        remoteDuringSave.value = false
        saveState.value = 'failed'
        refetchPrivate()
        useChatStore().notify($i18n.t('workspaceUi.stores.savePreferencesFailed'))
      },
    },
  )
  onScopeDispose(queue.dispose)
  function update(changes: Partial<ClientPreferences>) {
    revision.value++
    const next = sanitizePreferences({ ...preferences.value, ...changes })
    pending.value = next
    queryCache.cancelQueries({ key })
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

  /** A newer private event takes precedence over a still-pending save response. */
  function refreshFromEvent() {
    remoteRevision++
    if (pending.value) remoteDuringSave.value = true
    refetchPrivate()
  }

  function refetchPrivate() {
    const ticket = ++readGeneration
    remoteRefreshPending.value = true
    queryCache.cancelQueries({ key })
    void query
      .refetch()
      .then(() => {
        if (ticket === readGeneration && !query.error.value) remoteRefreshPending.value = false
      })
      .catch(() => {})
  }

  /** An in-flight remote refresh cannot grant an unconfirmed opt-in. */
  function allowsNsfw(networkId: string) {
    if (!ready.value || preferences.value.nsfwNetworks[networkId] !== true) return false
    if (remoteRefreshPending.value) return false
    if (!remoteDuringSave.value) return true
    return sanitizePreferences(query.data.value?.preferences).nsfwNetworks[networkId] === true
  }

  return {
    query,
    preferences,
    ready,
    revision,
    update,
    setIcon,
    saveState,
    refreshFromEvent,
    allowsNsfw,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(usePreferencesStore, import.meta.hot))
