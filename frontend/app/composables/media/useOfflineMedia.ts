import { onScopeDispose, ref, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import {
  offlineAccountRevision,
  offlineArchive,
  offlineRevision,
  offlineStatus,
} from '../../utils/offline-archive'

/** Disabling releases saved bytes; enabling checks this account before using the original URL. */
export function useOfflineMedia(
  raw: MaybeRefOrGetter<string>,
  enabled: MaybeRefOrGetter<boolean> = true,
) {
  const source = ref<string>()
  const saved = ref(false)
  const loading = ref(false)
  const contentType = ref('')
  const refs = ref<{ id: string; pinned: boolean }[]>([])
  let objectURL: string | undefined
  let requested = ''
  let accountRevision = offlineAccountRevision.value
  let generation = 0
  const fileRevision = ref(0)
  const release = () => {
    if (objectURL) URL.revokeObjectURL(objectURL)
    objectURL = undefined
  }
  const clear = () => {
    release()
    saved.value = false
    contentType.value = ''
    refs.value = []
  }
  watch(
    [() => toValue(raw), () => toValue(enabled)],
    ([url, active], _previous, cleanup) => {
      if (url && active) cleanup(offlineArchive.subscribeFile(url, () => fileRevision.value++))
    },
    { immediate: true, flush: 'sync' },
  )
  watch(
    [
      () => toValue(raw),
      () => toValue(enabled),
      offlineRevision,
      offlineAccountRevision,
      fileRevision,
      () => offlineStatus.value.offline,
    ],
    async ([url, active], previous, cleanup) => {
      const revision = ++generation
      const changed =
        requested !== url ||
        accountRevision !== offlineAccountRevision.value ||
        (active && !previous?.[1])
      requested = url
      accountRevision = offlineAccountRevision.value
      if (changed) clear()
      if (!active || !url) {
        clear()
        source.value = offlineStatus.value.offline ? undefined : url || undefined
        loading.value = false
        return
      }
      const controller = new AbortController()
      cleanup(() => {
        generation++
        controller.abort()
      })
      loading.value = changed || !source.value
      // Look for saved bytes before trying the network on the first render.
      // A failed original must not remove the preview before this read finishes.
      if (changed || offlineStatus.value.offline)
        source.value =
          objectURL ??
          (offlineArchive.available() || offlineStatus.value.offline ? undefined : url || undefined)
      try {
        const file = await offlineArchive.file(url, controller.signal)
        if (revision !== generation) return
        refs.value = file ? file.refs : []
        saved.value = !!(file && file.blob)
        contentType.value = file && file.blob ? file.blob.type : ''
        if (file && file.blob) {
          objectURL ??= URL.createObjectURL(file.blob)
          source.value = objectURL
        } else {
          release()
          source.value = offlineStatus.value.offline ? undefined : url || undefined
        }
      } catch {
        if (revision === generation) {
          if (!offlineArchive.available()) clear()
          source.value = objectURL ?? (offlineStatus.value.offline ? undefined : url || undefined)
        }
      } finally {
        if (revision === generation) loading.value = false
      }
    },
    { immediate: true, flush: 'sync' },
  )
  onScopeDispose(() => {
    generation++
    clear()
    source.value = undefined
    loading.value = false
  })
  return { source, saved, refs, contentType, loading }
}
