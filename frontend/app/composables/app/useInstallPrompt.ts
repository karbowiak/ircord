interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

// Chromium fires this once, early; keep it so settings can offer “Install” later.
const deferred = shallowRef<BeforeInstallPromptEvent | null>(null)
if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (event) => {
    event.preventDefault()
    deferred.value = event as BeforeInstallPromptEvent
  })
  window.addEventListener('appinstalled', () => (deferred.value = null))
}

/** The browser's “install app” prompt, where supported (Chromium on desktop and Android). */
export function useInstallPrompt() {
  return {
    get available() {
      return !!deferred.value
    },
    async prompt() {
      const event = deferred.value
      if (!event) return
      await event.prompt()
      await event.userChoice.catch(() => null)
      deferred.value = null
    },
  }
}
