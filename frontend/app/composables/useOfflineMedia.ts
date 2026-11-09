import { offlineArchive, offlineRevision, offlineStatus } from '../utils/offline-archive'

/** Object URLs are created only from this account's saved bytes, never from wire data. */
export function useOfflineMedia(raw: MaybeRefOrGetter<string>) {
  const source = ref<string>()
  const saved = ref(false)
  const loading = ref(true)
  const contentType = ref('')
  const refs = ref<{ id: string; pinned: boolean }[]>([])
  let objectURL: string | undefined
  let loaded = ''
  let requested = ''
  let generation = 0
  const release = () => {
    if (objectURL) URL.revokeObjectURL(objectURL)
    objectURL = undefined
    loaded = ''
  }
  watch(
    [() => toValue(raw), offlineRevision, () => offlineStatus.value.offline],
    async ([url]) => {
      const revision = ++generation
      const changed = requested !== url
      if (changed) loading.value = true
      requested = url
      if (loaded !== url) release()
      // Look for saved bytes before trying the network on the first render.
      // A failed original must not remove the preview before this read finishes.
      if (changed || offlineStatus.value.offline)
        source.value =
          objectURL ??
          (offlineArchive.available() || offlineStatus.value.offline ? undefined : url || undefined)
      saved.value = false
      contentType.value = ''
      refs.value = []
      try {
        const file = url && (await offlineArchive.file(url))
        if (revision !== generation) return
        refs.value = file ? file.refs : []
        saved.value = !!(file && file.blob)
        contentType.value = file && file.blob ? file.blob.type : ''
        if (file && file.blob) {
          if (!objectURL) {
            objectURL = URL.createObjectURL(file.blob)
            loaded = url
          }
          source.value = objectURL
        } else {
          release()
          source.value = offlineStatus.value.offline ? undefined : url || undefined
        }
      } catch {
        if (revision === generation) {
          if (!offlineArchive.available()) release()
          saved.value = false
          source.value = objectURL ?? (offlineStatus.value.offline ? undefined : url || undefined)
        }
      } finally {
        if (revision === generation) loading.value = false
      }
    },
    { immediate: true },
  )
  onBeforeUnmount(() => {
    generation++
    release()
  })
  return { source, saved, refs, contentType, loading }
}
