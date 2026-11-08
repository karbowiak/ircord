import { overlayOpen } from '../../shared/utils/focus'
import { canReloadSilently, isNewerBuild, type BuildInfo } from '../../shared/utils/update'

/** How long a tab must stay hidden before it may reload without asking. */
export const HIDDEN_GRACE_MS = 2 * 60_000
const RETRY_MS = 30_000

// Shared by every caller: the chat store triggers checks, the banner shows the result.
const latestId = ref('')
const dismissedId = ref('')
let lastCheck = 0
let hiddenSince = 0
let attached = false
let retryTimer: ReturnType<typeof setTimeout> | undefined
let build: (BuildInfo & { manifest: string }) | undefined

/** Text fields whose content is saved elsewhere (the composer's drafts) carry this attribute. */
const SAVED_FIELD = 'data-saved-draft'
const TEXT_FIELDS =
  'textarea, [contenteditable]:not([contenteditable="false"]), input:not([type=file], [type=checkbox], [type=radio], [type=hidden], [type=range], [type=color], [type=button], [type=submit], [type=reset])'

function unsavedInput() {
  return [...document.querySelectorAll<HTMLElement>(TEXT_FIELDS)].some((field) => {
    if (field.closest(`[${SAVED_FIELD}]`)) return false
    const value =
      field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement
        ? field.value
        : field.textContent
    return !!value?.trim()
  })
}

/**
 * Notices when the frontend server is running a newer build than this page and offers a reload.
 * Every release restarts the frontend, which drops the WebSocket, so the chat store checks on each
 * (re)connect; Nuxt's own periodic manifest poll and tab re-focus cover the rest. A visible tab
 * only shows the banner. A tab that has stayed hidden for a while reloads itself, but only when
 * nothing would be lost: no unacknowledged send, queued upload, open dialog or typed text.
 */
export function useAppUpdate() {
  // Nuxt context is read once on first use (the chat store's setup); the listeners, hook and
  // timers below run outside any setup context.
  if (!build && import.meta.client) {
    const config = useRuntimeConfig().app
    build = {
      id: config.buildId,
      manifest: `${config.cdnURL || ''}${config.buildAssetsDir}builds/latest.json`,
    }
    if (document.visibilityState === 'hidden') hiddenSince = Date.now()
    useNuxtApp().hooks.hook('app:manifest:update', (meta) => {
      if (build && isNewerBuild(build, meta)) found(meta.id)
    })
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') {
        hiddenSince = 0
        clearTimeout(retryTimer)
        if (Date.now() - lastCheck > 60_000) void check()
      } else {
        hiddenSince = Date.now()
        reloadIfSafe()
      }
    })
  }

  function found(id: string) {
    latestId.value = id
    reloadIfSafe()
  }

  /** Asks the frontend server which build it serves now. Failures are silent. */
  async function check() {
    if (!build) return
    lastCheck = Date.now()
    try {
      const latest = await $fetch<unknown>(build.manifest, {
        cache: 'no-store',
        query: { t: lastCheck },
      })
      const { id, timestamp } = (latest ?? {}) as { id?: unknown; timestamp?: unknown }
      if (id === build.id) {
        // This page is current again (a rollback, or a replica not yet updated): drop the notice,
        // and remember our own build time so an older replica is never mistaken for an update.
        if (typeof timestamp === 'number') build.timestamp = timestamp
        latestId.value = ''
        dismissedId.value = ''
        clearTimeout(retryTimer)
      } else if (isNewerBuild(build, latest)) found(latest.id)
    } catch {
      /* Offline or mid-deploy: the next reconnect or poll checks again. */
    }
  }

  /** Reloads to the new build, saving the composer drafts first. */
  function reload(silent = false) {
    useChatStore().flushDrafts()
    // A silent reload keeps Nuxt's loop guard, so a tab that lands on a not-yet-updated replica
    // does not reload again immediately; a click always reloads.
    reloadNuxtApp(silent ? { ttl: 60_000 } : { force: true })
  }

  function reloadIfSafe() {
    clearTimeout(retryTimer)
    if (!attached || !latestId.value || document.visibilityState !== 'hidden') return
    const hiddenFor = hiddenSince ? Date.now() - hiddenSince : 0
    if (hiddenFor < HIDDEN_GRACE_MS) {
      retryTimer = setTimeout(reloadIfSafe, HIDDEN_GRACE_MS - hiddenFor)
      return
    }
    const chat = useChatStore()
    const state = {
      hidden: true,
      hiddenLongEnough: true,
      outboxEmpty: Object.values(chat.outbox).every((list) => !list.length),
      uploadsPending: hasQueuedUploads(),
      overlayOpen: overlayOpen(),
      unsavedInput: unsavedInput(),
    }
    if (canReloadSilently(state)) reload(true)
    // Hidden but busy: look again later; the send may finish or the dialog may be closed.
    else retryTimer = setTimeout(reloadIfSafe, RETRY_MS)
  }

  return {
    /** A newer build is live and the user has not dismissed the notice. */
    get available() {
      return !!latestId.value && latestId.value !== dismissedId.value
    },
    /** The build id the server announced, or '' when this page is current. */
    get latestId() {
      return latestId.value
    },
    check,
    reload: () => reload(),
    /** Hides the notice until an even newer build appears. */
    dismiss: () => (dismissedId.value = latestId.value),
    /** The signed-in workspace is running; only then may a hidden tab reload itself. */
    attach: () => (attached = true),
    detach: () => {
      attached = false
      clearTimeout(retryTimer)
    },
  }
}
