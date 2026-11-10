/** Open modal dialogs, oldest first. Each closes through its own close path, restoring focus. */
const open = shallowReactive<{ close: () => void }[]>([])

/** The dialogs IRCord has open (AppDialog, the media viewer), for shortcuts and Escape. */
export function useDialogStack() {
  return {
    /** Adds a dialog; the returned function removes it again. */
    register(close: () => void) {
      const entry = { close }
      open.push(entry)
      return () => {
        const index = open.indexOf(entry)
        if (index >= 0) open.splice(index, 1)
      }
    },
    get count() {
      return open.length
    },
    /** Closes the newest dialog. False when none is open. */
    closeTop() {
      const top = open.at(-1)
      top?.close()
      return !!top
    },
  }
}
