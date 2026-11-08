import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import {
  emptyLibrary,
  libraryLimits,
  pickQuickReactions,
  recentEmojiIds,
  recordUsage,
  sanitizeLibrary,
  savedGif,
  type PickerLibrary,
} from '../../shared/library'
import type { EmojiItem, GifItem } from '../../shared/types/media'

const localKey = () => `ircord.library.${useAuthStore().account?.id ?? 'signed-out'}`
const key = ['library']

/**
 * Saved GIFs, emoji usage and skin tone. The account copy on the server is the source
 * of truth; a local mirror makes the picker instant on load and keeps working offline.
 */
export const useLibraryStore = defineStore('library', () => {
  const queryCache = useQueryCache()
  const chat = useChatStore()
  const query = useQuery({
    key,
    query: () => appClient.library(),
    placeholderData: () => readLocal(localKey(), sanitizeLibrary),
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  })
  const library = computed(() => query.data.value ?? emptyLibrary())
  watch(library, (value) => query.data.value && writeLocal(localKey(), value))

  const set = (change: (value: PickerLibrary) => PickerLibrary) =>
    queryCache.setQueryData(key, change(library.value))
  const settle = (saved: PickerLibrary) => {
    queryCache.cancelQueries({ key })
    queryCache.setQueryData(key, saved)
  }
  const fail = (previous: PickerLibrary) => {
    queryCache.setQueryData(key, previous)
    chat.notify('Couldn’t save that change. Please try again.')
  }

  const gifIds = computed(() => new Set(library.value.gifs.map((gif) => gif.id)))
  const isFavoriteGif = (gif: GifItem) => gifIds.value.has(gif.id)
  /** Emoji ids, most used first (already ordered by score). */
  const frequentEmoji = computed(() => library.value.emojiUsage.map((u) => u.id))
  const usageRank = computed(() => new Map(frequentEmoji.value.map((id, i) => [id, i])))
  /** Emoji ids, most recently picked first: the message toolbar's quick reactions. */
  const recentEmoji = computed(() => recentEmojiIds(library.value.emojiUsage))
  const media = useMediaStore()
  // Recent emoji resolve against the catalogues; without history the defaults need none.
  watch(
    () => recentEmoji.value.length > 0,
    (any) => any && media.needAll(),
    { immediate: true },
  )
  /** Five quick reactions (the hover toolbar shows the first three). */
  const quickReactions = computed(() => pickQuickReactions(recentEmoji.value, media.byId, 5))

  const gifMutation = useMutation({
    mutation: ({ gif, saved }: { gif: GifItem; saved: boolean }) =>
      saved
        ? appClient.saveGif(savedGif(gif))
        : appClient.removeGif(gif.id),
    onMutate: ({ gif, saved }) => {
      const previous = library.value
      set((value) => ({
        ...value,
        gifs: saved
          ? [savedGif(gif), ...value.gifs.filter((x) => x.id !== gif.id)].slice(
              0,
              libraryLimits.gifs,
            )
          : value.gifs.filter((x) => x.id !== gif.id),
      }))
      return { previous }
    },
    onSuccess: settle,
    onError: (_error, _vars, { previous }) => previous && fail(previous),
  })
  // Emoji picks are counted locally at once and sent as increments after a pause.
  let pendingUses: Record<string, number> = {}
  /** The ids in `pendingUses`, oldest pick first (object key order can't hold numeric ids). */
  let pendingOrder: string[] = []
  let pendingTone: string | undefined
  let flushTimer: ReturnType<typeof setTimeout> | undefined
  function schedule() {
    clearTimeout(flushTimer)
    flushTimer = setTimeout(flush, 1500)
  }
  async function flush() {
    const body = {
      uses: pendingUses,
      recent: pendingOrder,
      ...(pendingTone ? { skinTone: pendingTone } : {}),
    }
    pendingUses = {}
    pendingOrder = []
    pendingTone = undefined
    try {
      settle(await appClient.updateLibrary(body))
    } catch {
      /* Usage stats are best effort; the local count still applies. */
    }
  }
  function useEmoji(item: EmojiItem) {
    pendingUses[item.id] = (pendingUses[item.id] ?? 0) + 1
    pendingOrder = [...pendingOrder.filter((id) => id !== item.id), item.id]
    set((value) => ({ ...value, emojiUsage: recordUsage(value.emojiUsage, { [item.id]: 1 }) }))
    schedule()
  }
  function setSkinTone(tone: string) {
    pendingTone = tone
    set((value) => ({ ...value, skinTone: tone }))
    schedule()
  }
  window.addEventListener('pagehide', () => {
    if (!Object.keys(pendingUses).length && !pendingTone) return
    clearTimeout(flushTimer)
    void flush()
  })

  void migrateLegacyFavorites()
  /** Favourites used to be per-browser and kept only slugs. Move them into the account once. */
  async function migrateLegacyFavorites() {
    try {
      const legacy = Object.keys(localStorage).filter(
        (name) => name === `ircord.picker.${useAuthStore().account?.id}.v1`,
      )
      if (!legacy.length) return
      const old = JSON.parse(localStorage.getItem(legacy[0]!) || '{}')
      const slugs: string[] = (old.favorites ?? [])
        .map((f: { slug?: string }) => f.slug)
        .filter(Boolean)
      const appKey = useMediaStore().appKey
      for (let i = 0; i < slugs.length && appKey; i += 30)
        for (const gif of await klipyItems(appKey, slugs.slice(i, i + 30)))
          if (!isFavoriteGif(gif)) await gifMutation.mutateAsync({ gif, saved: true })
      if (Array.isArray(old.recentEmoji))
        for (const id of old.recentEmoji.slice(0, libraryLimits.emojiUsage))
          if (typeof id === 'string') pendingUses[id] = 1
      if (old.skinTone) pendingTone = sanitizeLibrary({ skinTone: old.skinTone }).skinTone
      schedule()
      legacy.forEach((name) => localStorage.removeItem(name))
    } catch {
      /* Try again next load. */
    }
  }

  return {
    query,
    library,
    isFavoriteGif,
    frequentEmoji,
    usageRank,
    recentEmoji,
    quickReactions,
    toggleGif: (gif: GifItem) => gifMutation.mutate({ gif, saved: !isFavoriteGif(gif) }),
    useEmoji,
    setSkinTone,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useLibraryStore, import.meta.hot))
