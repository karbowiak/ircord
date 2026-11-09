import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import {
  compileAppearance,
  defaultAppearance,
  sanitizeAppearance,
  type Appearance,
} from '../../shared/appearance'
import { compiledAppearanceKey } from '../../shared/appearance-boot'

interface LocalAppearance {
  settings: Appearance
  /** Server revision these settings came from or were last saved as. */
  revision: string | null
  /** Edited here and not confirmed by the server yet. */
  dirty: boolean
  /** Follow the account's appearance; when off this device keeps its own look. */
  sync: boolean
}
const localKey = 'ircord.appearance'
const readStored = (): LocalAppearance =>
  readLocal(localKey, (value) => {
    const v = (value && typeof value === 'object' ? value : {}) as Partial<LocalAppearance>
    return {
      settings: sanitizeAppearance(v.settings ?? defaultAppearance),
      revision: typeof v.revision === 'string' ? v.revision : null,
      dirty: v.dirty === true,
      sync: v.sync !== false,
    }
  })

/**
 * Appearance is local-first: it applies instantly from this browser's copy, then reconciles with
 * the account copy on the server so one look follows the user to every device.
 */
export const useAppearanceStore = defineStore('appearance', () => {
  const local = reactive(readStored())
  const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const queryCache = useQueryCache()
  const remote = useQuery({
    key: ['appearance'],
    query: () => appClient.appearance(),
    enabled: () => local.sync && useAuthStore().status === 'signed-in',
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  })

  const queue = createLatestQueue((settings: Appearance) => appClient.saveAppearance(settings), {
    delay: 400,
    onSaved: (_value, saved) => {
      queryCache.setQueryData(['appearance'], saved)
      local.revision = saved.updatedAt
      local.dirty = false
      saveState.value = 'saved'
      persist()
    },
    onError: () => (saveState.value = 'error'),
  })

  // Server copy arrived: unsaved local edits win, otherwise adopt the account's look.
  watch(
    () => remote.data.value,
    (stored) => {
      if (!stored || !local.sync) return
      if (local.dirty || !stored.settings) {
        saveState.value = 'saving'
        queue.push(local.settings)
      } else if (stored.updatedAt !== local.revision) {
        local.settings = sanitizeAppearance(stored.settings)
        local.revision = stored.updatedAt
        persist()
      }
    },
  )

  function update(changes: Partial<Appearance>) {
    local.settings = sanitizeAppearance({ ...local.settings, ...changes })
    local.dirty = true
    persist()
    if (local.sync) {
      saveState.value = 'saving'
      queue.push(local.settings)
    }
  }
  function setSync(value: boolean) {
    local.sync = value
    persist()
    if (value) void remote.refetch()
  }

  // ---- Applying to the document ---------------------------------------------------------------

  const prefersLight = useMediaQuery('(prefers-color-scheme: light)')
  const prefersReduced = useMediaQuery('(prefers-reduced-motion: reduce)')

  const mode = computed(() =>
    local.settings.mode === 'system'
      ? prefersLight.value
        ? 'light'
        : 'dark'
      : local.settings.mode,
  )
  const compiled = computed(() => compileAppearance(local.settings, mode.value))
  const reducedMotion = computed(() =>
    local.settings.motion === 'system' ? prefersReduced.value : local.settings.motion === 'reduced',
  )
  /** Animated GIFs and emotes play only when the user wants them and motion is not reduced. */
  const animate = computed(() => local.settings.animateMedia && !reducedMotion.value)

  function persist() {
    writeLocal(localKey, local)
    // Both modes, so the boot script can follow the system preference before the app loads.
    writeLocal(compiledAppearanceKey, {
      mode: local.settings.mode,
      dark: pickDocument(compileAppearance(local.settings, 'dark')),
      light: pickDocument(compileAppearance(local.settings, 'light')),
    })
  }
  const pickDocument = ({ css, attrs }: { css: string; attrs: Record<string, string> }) => ({
    css,
    attrs,
  })

  watchEffect(() => {
    const { css, attrs, background } = compiled.value
    const root = document.documentElement
    for (const [name, value] of Object.entries(attrs)) root.setAttribute(name, value)
    root.dataset.motion = reducedMotion.value ? 'reduced' : 'full'
    root.style.colorScheme = mode.value
    let style = document.getElementById('ircord-theme')
    if (!style) {
      style = document.createElement('style')
      style.id = 'ircord-theme'
      document.head.appendChild(style)
    }
    style.textContent = css
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', background)
  })

  // Another tab changed the appearance.
  useEventListener(window, 'storage', (event) => {
    if (event.key === localKey) Object.assign(local, readStored())
  })
  persist()

  return {
    settings: computed(() => local.settings),
    sync: computed(() => local.sync),
    saveState,
    mode,
    tokens: computed(() => compiled.value.tokens),
    reducedMotion,
    animate,
    update,
    setSync,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useAppearanceStore, import.meta.hot))
