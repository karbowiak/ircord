/**
 * Starts the stores every screen needs and registers the media cache. The chat store starts
 * with the workspace, after sign-in.
 */
export default defineNuxtPlugin(() => {
  useAppearanceStore()
  useLayoutStore()
  useAuthStore()
  // Captures the browser's install prompt, which fires once shortly after load.
  useInstallPrompt()
  if ('serviceWorker' in navigator)
    navigator.serviceWorker.register('/sw.js').catch(() => {
      /* The app works without the media cache and push. */
    })
})
