/**
 * A `before-close` guard for a dialog with a long form. While the form has changes, closing by
 * Escape, an outside click, the close button or browser Back asks first; only a confirmed
 * discard closes the dialog. `close` runs after that confirmation.
 */
export function useDiscardGuard(dirty: () => boolean, close: () => void) {
  const { t } = useI18n()
  const { confirm } = usePrompt()
  let confirmed = false
  let asking = false
  return () => {
    if (confirmed || !dirty()) return true
    if (!asking) {
      asking = true
      void confirm({
        title: t('prompt.discardTitle'),
        body: t('prompt.discardBody'),
        confirm: t('prompt.discardAction'),
        danger: true,
      }).then((discard) => {
        asking = false
        if (!discard) return
        confirmed = true
        close()
      })
    }
    return false
  }
}
