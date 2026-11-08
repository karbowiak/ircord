import type { Message } from '../../shared/types/chat'

/** The message being edited in place, shared by its row, menus and the composer. */
const editingId = ref<string>()

export function useMessageEditing() {
  return {
    editingId: readonly(editingId),
    start: (message: Message) => {
      editingId.value = message.id
    },
    stop: () => {
      editingId.value = undefined
    },
  }
}
