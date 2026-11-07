/** Route plain typing to the composer while preserving dialogs, selection and shortcuts. */
export function useTypeToCompose(input: Ref<HTMLTextAreaElement | null>) {
  function keydown(event: KeyboardEvent) {
    const target = event.target instanceof HTMLElement ? event.target : null
    if (
      !input.value ||
      input.value.disabled ||
      event.defaultPrevented ||
      event.isComposing ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey ||
      event.key.length !== 1 ||
      event.key === '?' ||
      target?.closest('input, textarea, select, [contenteditable]') ||
      (event.key === ' ' && target?.closest('button, a, summary, [role="button"]')) ||
      document.querySelector(
        'dialog[open], [popover]:popover-open, [role="menu"], [role="listbox"]',
      ) ||
      !window.getSelection()?.isCollapsed
    )
      return
    // Focus before the browser's default text insertion, retaining the first key
    // and the draft's caret without manually synthesizing text or input events.
    input.value.focus({ preventScroll: true })
  }
  onMounted(() => window.addEventListener('keydown', keydown, true))
  onBeforeUnmount(() => window.removeEventListener('keydown', keydown, true))
}
