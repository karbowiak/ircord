import { computed, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'

/** A reveal belongs to this mounted card and is cleared when its identity or opt-in changes. */
export function useSensitiveReveal(
  identity: MaybeRefOrGetter<string>,
  networkIds: MaybeRefOrGetter<readonly string[]>,
) {
  const preferences = usePreferencesStore()
  const revealed = ref(false)
  const choices = computed(() =>
    Object.fromEntries(
      toValue(networkIds).map((id) => [
        id,
        preferences.preferences.showNsfw || preferences.preferences.nsfwNetworks[id] === true,
      ]),
    ),
  )
  watch(
    () => toValue(identity),
    () => {
      revealed.value = false
    },
    { flush: 'sync' },
  )
  watch(
    choices,
    (next, previous) => {
      if (Object.keys(previous).some((id) => previous[id] && !next[id])) revealed.value = false
    },
    { flush: 'sync' },
  )
  return {
    revealed,
    reveal: () => {
      revealed.value = true
    },
  }
}
