import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery } from '@pinia/colada'
import type { EmojiItem } from '../../shared/types/media'
import { emojiCatalogQuery, instanceEmotesQuery, standardEmojiQuery } from '../queries/media'
import { customShortcodes, instanceEmojiItems } from '../../shared/utils/media'

/** Emoji catalogues, loaded on first need and cached for the session. */
export const useMediaStore = defineStore('media', () => {
  const appKey = useRuntimeConfig().public.klipyApiKey as string
  const wantCustom = ref(false)
  const wantStandard = ref(false)
  const catalog = useQuery({ ...emojiCatalogQuery, enabled: () => wantCustom.value })
  const standardQuery = useQuery({ ...standardEmojiQuery, enabled: () => wantStandard.value })
  // Without a Go backend (demo), the shared catalog is simply empty.
  const instanceQuery = useQuery({ ...instanceEmotesQuery, enabled: () => wantCustom.value })

  const instance = computed(() => instanceEmojiItems(instanceQuery.data.value))
  const bundled = computed(() => catalog.data.value?.items ?? [])
  const custom = computed(() => [...instance.value, ...bundled.value])
  const standard = computed(() => standardQuery.data.value ?? [])
  const emojis = computed(() => [...custom.value, ...standard.value])
  const byId = computed(() => new Map(emojis.value.map((item) => [item.id, item])))
  // The backend catalog wins over the bundled workspace set, which wins over 7TV globals.
  const standardNames = computed(
    () => new Set(standard.value.flatMap((item) => [item.shortcode, ...item.aliases])),
  )
  const byShortcode = computed(() =>
    customShortcodes(bundled.value, instance.value, instanceQuery.data.value, standardNames.value),
  )

  /** Every emoji by shortcode or alias, custom ones winning; used to convert typed `:name:`. */
  const byShortcodeAll = computed(() => {
    const map = new Map<string, EmojiItem>()
    // Aliases first, so a real shortcode (fire) is never shadowed by another emoji's alias.
    for (const item of standard.value) for (const code of item.aliases) map.set(code, item)
    for (const item of standard.value) map.set(item.shortcode, item)
    for (const [code, item] of byShortcode.value) map.set(code, item)
    return map
  })

  return {
    appKey,
    gifsReady: !!appKey,
    catalog,
    instanceQuery,
    standardQuery,
    custom,
    standard,
    emojis,
    byId,
    byShortcode,
    byShortcodeAll,
    /** Message text references custom emoji: make sure the catalogue is loading. */
    needCustom: () => (wantCustom.value = true),
    /** The picker opened: load everything. */
    needAll: () => {
      wantCustom.value = true
      wantStandard.value = true
    },
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useMediaStore, import.meta.hot))
