/** Drag files anywhere over the conversation to attach them (not in server windows). */
export function useFileDrop() {
  const chat = useChatStore()
  const uploads = useUploads()
  const dragging = ref(false)
  let depth = 0
  const hasFiles = (event: DragEvent) => event.dataTransfer?.types.includes('Files') ?? false
  return {
    dragging,
    enter(event: DragEvent) {
      if (!hasFiles(event) || !uploads.capability.value || chat.serverView) return
      depth++
      dragging.value = true
    },
    leave() {
      depth = Math.max(0, depth - 1)
      if (!depth) dragging.value = false
    },
    drop(event: DragEvent) {
      depth = 0
      dragging.value = false
      if (event.dataTransfer?.files.length && !chat.serverView)
        void uploads.add(event.dataTransfer.files)
    },
  }
}
