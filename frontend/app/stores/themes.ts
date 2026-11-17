import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import { compileAppearance, type Appearance } from '../../shared/appearance'
import type { SchemeMode } from '../../shared/schemes'
import { compiledThemeKey, safeModePattern, themeDraftKey } from '../../shared/appearance-boot'
import { filterThemeCss, type FilteredCss } from '../../shared/theme-css'
import {
  exportTheme,
  importTheme,
  newThemeId,
  sanitizeTheme,
  themeFileName,
  themeFormat,
  themeSupported,
  type Theme,
  type ThemeLibrary,
} from '../../shared/themes'

/** Seconds a theme with custom CSS runs before it reverts, unless the person keeps it. */
export const trialSeconds = 20

interface CompiledDocument {
  css: string
  attrs: Record<string, string>
}
/** The pre-paint copy of the kept theme (appearance-boot.ts reads it before the app loads). */
interface ThemeCache {
  account: string
  theme: Theme
  dark: CompiledDocument
  light: CompiledDocument
  userCss: string
}
const compiledDocument = (value: unknown): value is CompiledDocument =>
  !!value &&
  typeof (value as CompiledDocument).css === 'string' &&
  typeof (value as CompiledDocument).attrs === 'object'
const readCache = () =>
  readLocal<ThemeCache | null>(compiledThemeKey, (value) => {
    const v = (value && typeof value === 'object' ? value : {}) as Partial<ThemeCache>
    const theme = sanitizeTheme(v.theme)
    return typeof v.account === 'string' &&
      theme &&
      compiledDocument(v.dark) &&
      compiledDocument(v.light)
      ? {
          account: v.account,
          theme,
          dark: v.dark,
          light: v.light,
          userCss: String(v.userCss ?? ''),
        }
      : null
  })
interface SessionDraft {
  theme: Theme
  base: string | null
}
const readSessionDraft = () =>
  readLocal<SessionDraft | null>(
    themeDraftKey,
    (value) => {
      const v = (value && typeof value === 'object' ? value : {}) as Partial<SessionDraft>
      const theme = sanitizeTheme(v.theme)
      return theme ? { theme, base: typeof v.base === 'string' ? v.base : null } : null
    },
    'session',
  )

interface Trial {
  theme: Theme
  deadline: number
  keep: () => unknown
  revert: () => unknown
}

/**
 * What this store needs from the appearance store. The appearance store reads this one while it
 * is being set up, so it links itself in (connectAppearance) instead of being looked up here.
 */
interface AppearanceLink {
  settings: () => Appearance
  update: (changes: Partial<Appearance>) => void
}

const same = (a: Theme | null | undefined, b: Theme | null | undefined) =>
  JSON.stringify(a ?? null) === JSON.stringify(b ?? null)

/**
 * Personal themes (#88): the account's theme library, the studio's draft, and the safety net for
 * custom CSS (the keep-or-revert countdown, protected screens, safe mode and the pause shortcut).
 * The appearance store compiles whatever `applied` says onto the document.
 */
export const useThemesStore = defineStore('themes', () => {
  const auth = useAuthStore()
  const queryCache = useQueryCache()
  const version = useAppConfig().release.version as string
  const cache = shallowRef(import.meta.client ? readCache() : null)

  /** The account themes belong to. While the session is checked, the pre-paint copy is trusted. */
  const account = computed(() =>
    auth.status === 'signed-in'
      ? (auth.account?.id ?? null)
      : auth.status === 'checking' || auth.status === 'error'
        ? (cache.value?.account ?? null)
        : null,
  )
  const query = useQuery({
    key: ['themes'],
    query: () => appClient.themes(),
    enabled: () => auth.status === 'signed-in',
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  })
  const library = computed<ThemeLibrary | null>(() => query.data.value ?? null)
  const themes = computed(() => library.value?.themes ?? [])
  const setLibrary = (saved: ThemeLibrary) => queryCache.setQueryData(['themes'], saved)

  // ---- What applies -----------------------------------------------------------------------------

  /** `?safe` in the address: this visit runs without the personal theme. */
  const safe = ref(import.meta.client && safeModePattern.test(location.search))
  /** The pause shortcut: like safe mode, for this visit. */
  const paused = ref(false)
  /** The studio's "Show without theme". */
  const compare = ref(false)
  /** Screens that never run custom CSS while open: Settings, sign-in, confirmations. */
  const protectedBy = reactive(new Set<string>())
  const draft = ref<Theme | null>(null)
  /** The mode the studio previews, when it differs from the saved one. */
  const previewMode = ref<SchemeMode | null>(null)
  const trial = shallowRef<Trial | null>(null)

  const link = shallowRef<AppearanceLink | null>(null)
  function connectAppearance(value: AppearanceLink) {
    link.value = value
  }
  const appearanceStore = () => {
    if (!link.value) throw new Error('The appearance store is not connected.')
    return link.value
  }
  const keptId = computed(() => link.value?.settings().theme ?? null)
  /** The theme the account chose, as far as this device knows it. */
  const kept = computed<Theme | null>(() => {
    const id = keptId.value
    if (!id || !account.value) return null
    const list = library.value
      ? themes.value
      : cache.value?.account === account.value
        ? [cache.value.theme]
        : []
    const theme = list.find((item) => item.id === id)
    return theme && themeSupported(theme) ? theme : null
  })
  const suspended = computed(() => safe.value || paused.value)
  /** The theme the document shows right now. */
  const applied = computed<Theme | null>(() => {
    if (!account.value || compare.value) return null
    if (draft.value) return draft.value
    if (suspended.value) return null
    return trial.value?.theme ?? kept.value
  })

  /** The operator's switch, from the bootstrap capabilities (set by the workspace). */
  const operatorCss = ref(true)
  const accountCss = computed(() => library.value?.customCss !== false)
  const cssAllowed = computed(() => operatorCss.value && accountCss.value)
  const protectedScreen = computed(() => protectedBy.size > 0)

  // Filtering is pure; remember the last few results so typing in the studio stays cheap.
  const filtered = new Map<string, FilteredCss>()
  function filter(css: string): FilteredCss {
    const hit = filtered.get(css)
    if (hit) return hit
    const result = filterThemeCss(css)
    filtered.set(css, result)
    if (filtered.size > 8) filtered.delete(filtered.keys().next().value!)
    return result
  }
  const appliedCss = computed(() => (applied.value?.css.trim() ? filter(applied.value.css) : null))
  /** The filtered custom CSS that is on the page now, or ''. */
  const userCss = computed(() =>
    cssAllowed.value && !protectedScreen.value ? (appliedCss.value?.css ?? '') : '',
  )
  /** Known variables the kept theme's CSS sets on the root, for "Set by your theme's CSS". */
  const cssVariables = computed(() =>
    cssAllowed.value && kept.value?.css.trim()
      ? new Set(filter(kept.value.css).report.rootVariables)
      : new Set<string>(),
  )

  function protect(reason: string, on: boolean) {
    if (on) protectedBy.add(reason)
    else protectedBy.delete(reason)
  }

  // ---- Pre-paint copy ---------------------------------------------------------------------------

  // Only a kept choice reaches the pre-paint copy: never a draft or a theme on trial.
  watchEffect(() => {
    if (!import.meta.client) return
    if (auth.status === 'signed-out') {
      removeCache()
      return
    }
    if (auth.status !== 'signed-in' || !account.value || trial.value || !link.value) return
    const theme = kept.value
    if (!theme) {
      if (library.value || keptId.value === null) removeCache()
      return
    }
    const settings = link.value.settings()
    const pick = ({ css, attrs }: CompiledDocument) => ({ css, attrs })
    const next: ThemeCache = {
      account: account.value,
      theme,
      dark: pick(compileAppearance(settings, 'dark', theme)),
      light: pick(compileAppearance(settings, 'light', theme)),
      userCss: cssAllowed.value && theme.css.trim() ? filter(theme.css).css : '',
    }
    writeLocal(compiledThemeKey, next)
    cache.value = next
  })
  function removeCache() {
    try {
      localStorage.removeItem(compiledThemeKey)
    } catch {
      // Storage unavailable: nothing was kept.
    }
    cache.value = null
  }

  // ---- Keep or revert ---------------------------------------------------------------------------

  const now = ref(Date.now())
  let ticker: ReturnType<typeof setInterval> | undefined
  const remaining = computed(() =>
    trial.value ? Math.max(0, Math.ceil((trial.value.deadline - now.value) / 1000)) : 0,
  )
  function startTrial(value: Omit<Trial, 'deadline'>) {
    stopTrial()
    now.value = Date.now()
    trial.value = { ...value, deadline: now.value + trialSeconds * 1000 }
    // The timer reverts even if the bar were somehow hidden.
    ticker = setInterval(() => {
      now.value = Date.now()
      if (trial.value && now.value >= trial.value.deadline) void revertTrial()
    }, 250)
  }
  function stopTrial() {
    clearInterval(ticker)
    ticker = undefined
    trial.value = null
  }
  async function keepTrial() {
    const current = trial.value
    stopTrial()
    await current?.keep()
  }
  async function revertTrial() {
    const current = trial.value
    stopTrial()
    await current?.revert()
  }
  onScopeDispose(stopTrial)

  /** Applies a theme. One with custom CSS runs on trial first; the result says which. */
  function apply(theme: Theme): 'applied' | 'trial' | 'unsupported' {
    if (!themeSupported(theme)) return 'unsupported'
    const appearance = appearanceStore()
    safe.value = false
    paused.value = false
    if (!theme.css.trim() || !cssAllowed.value) {
      appearance.update({ theme: theme.id })
      return 'applied'
    }
    startTrial({ theme, keep: () => appearance.update({ theme: theme.id }), revert: () => {} })
    return 'trial'
  }
  /** "Use built-in scheme": the theme is set to none; the library is untouched. */
  function useBuiltIn() {
    stopTrial()
    safe.value = false
    paused.value = false
    appearanceStore().update({ theme: null })
  }
  function resume() {
    safe.value = false
    paused.value = false
  }
  function togglePause() {
    if (!kept.value && !paused.value) return false
    paused.value = !paused.value
    return true
  }

  // ---- Library ----------------------------------------------------------------------------------

  const stamp = (theme: Theme): Theme => ({ ...theme, format: themeFormat, madeWith: version })
  async function save(theme: Theme, expected: string | null, force = false) {
    const saved = await appClient.saveTheme(stamp(theme), expected, force)
    setLibrary(saved)
    return saved.themes.find((item) => item.id === theme.id) ?? null
  }
  async function duplicate(theme: Theme, name: string) {
    return save({ ...theme, id: newThemeId(), name, updatedAt: '' }, null)
  }
  async function remove(theme: Theme) {
    const appearance = appearanceStore()
    // Deleting the theme in use falls back to the scheme it was based on.
    if (keptId.value === theme.id) {
      stopTrial()
      appearance.update({ theme: null, schemeDark: theme.base.dark, schemeLight: theme.base.light })
    }
    setLibrary(await appClient.deleteTheme(theme.id))
  }
  async function setCustomCss(value: boolean) {
    setLibrary(await appClient.setCustomCss(value))
  }
  /** Adds an imported theme to the library, not applied. */
  async function importFrom(raw: string) {
    const result = importTheme(raw, version)
    if (!result.ok) return result
    const saved = (await save(result.theme, null)) ?? result.theme
    return { ok: true as const, theme: saved, report: filter(saved.css).report }
  }
  function download(theme: Theme) {
    const blob = new Blob([JSON.stringify(exportTheme(theme), null, 2)], {
      type: 'application/json',
    })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = themeFileName(theme)
    link.click()
    setTimeout(() => URL.revokeObjectURL(link.href), 1000)
  }

  // ---- Studio -----------------------------------------------------------------------------------

  /** The version the draft started from; null for a new theme. */
  const studioBase = ref<string | null>(null)
  const studioNew = ref(false)
  /** An unsaved draft for the same theme from earlier in this session, offered back. */
  const restorable = shallowRef<Theme | null>(null)
  /** The stored copy, when the theme changed on another device since the draft started. */
  const conflict = shallowRef<Theme | null>(null)
  const saving = ref(false)
  const saveError = ref('')

  function open(theme: Theme, isNew = false) {
    draft.value = JSON.parse(JSON.stringify(theme)) as Theme
    studioBase.value = isNew ? null : theme.updatedAt
    studioNew.value = isNew
    conflict.value = null
    saveError.value = ''
    compare.value = false
    const previous = readSessionDraft()
    restorable.value =
      previous && previous.theme.id === theme.id && !same(previous.theme, theme)
        ? previous.theme
        : null
  }
  function edit(theme: Theme) {
    open(theme)
  }
  function create(name: string) {
    const appearance = appearanceStore().settings()
    const base = kept.value?.base ?? { dark: appearance.schemeDark, light: appearance.schemeLight }
    open(
      {
        format: themeFormat,
        id: newThemeId(),
        name,
        base: { ...base },
        colors: {},
        background: null,
        css: '',
        madeWith: version,
        updatedAt: '',
      },
      true,
    )
  }
  function restoreDraft() {
    if (restorable.value) draft.value = JSON.parse(JSON.stringify(restorable.value)) as Theme
    restorable.value = null
  }
  function closeStudio() {
    draft.value = null
    previewMode.value = null
    compare.value = false
    conflict.value = null
    restorable.value = null
    writeLocal(themeDraftKey, null, 'session')
  }
  // Keep the draft text for the session, so a reload does not lose work.
  let draftTimer: ReturnType<typeof setTimeout> | undefined
  watch(
    draft,
    (value) => {
      if (!value || !import.meta.client) return
      clearTimeout(draftTimer)
      draftTimer = setTimeout(
        () => writeLocal(themeDraftKey, { theme: value, base: studioBase.value }, 'session'),
        300,
      )
    },
    { deep: true },
  )

  async function saveDraft(force = false) {
    const theme = draft.value
    if (!theme || saving.value) return false
    const previous = themes.value.find((item) => item.id === theme.id) ?? null
    const isNew = studioNew.value
    const base = studioBase.value
    saveError.value = ''
    // New CSS for the theme in use runs on the countdown first, and only Keep saves it. Until
    // then the account keeps the previous version, so a reload during the countdown also
    // returns to it.
    if (
      !isNew &&
      previous &&
      keptId.value === theme.id &&
      cssAllowed.value &&
      theme.css.trim() &&
      theme.css !== previous.css
    ) {
      const edited = JSON.parse(JSON.stringify(theme)) as Theme
      closeStudio()
      startTrial({
        theme: stamp(edited),
        keep: () => commit(edited, base, force),
        revert: () => {},
      })
      return true
    }
    saving.value = true
    try {
      const saved = await save(theme, base, force)
      closeStudio()
      // A new theme is what the person just made to use: apply it, on trial when it has CSS.
      if (saved && isNew) apply(saved)
      return true
    } catch (error) {
      saveFailed(error)
      return false
    } finally {
      saving.value = false
    }
  }
  /** Keep after editing the theme in use: save it now, or reopen the studio with the problem. */
  async function commit(edited: Theme, base: string | null, force: boolean) {
    saving.value = true
    try {
      await save(edited, base, force)
    } catch (error) {
      open(edited)
      studioBase.value = base
      saveFailed(error)
    } finally {
      saving.value = false
    }
  }
  function saveFailed(error: unknown) {
    const theirs = sanitizeTheme(
      (error as { data?: { data?: { theirs?: unknown } } }).data?.data?.theirs,
    )
    if (errorStatus(error) === 412 && theirs) conflict.value = theirs
    else saveError.value = errorText(error)
  }
  /** "Load theirs": continue from the copy saved elsewhere. */
  function loadTheirs() {
    if (!conflict.value) return
    draft.value = JSON.parse(JSON.stringify(conflict.value)) as Theme
    studioBase.value = conflict.value.updatedAt
    conflict.value = null
  }

  return {
    connectAppearance,
    library,
    themes,
    loading: computed(() => query.isLoading.value),
    kept,
    applied,
    safe,
    paused,
    suspended,
    compare,
    draft,
    previewMode,
    trial,
    remaining,
    operatorCss,
    accountCss,
    cssAllowed,
    protectedScreen,
    userCss,
    cssVariables,
    version,
    filter,
    protect,
    apply,
    useBuiltIn,
    resume,
    togglePause,
    keepTrial,
    revertTrial,
    save,
    duplicate,
    remove,
    setCustomCss,
    importFrom,
    download,
    studioNew,
    restorable,
    conflict,
    saving,
    saveError,
    edit,
    create,
    restoreDraft,
    closeStudio,
    saveDraft,
    loadTheirs,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useThemesStore, import.meta.hot))
