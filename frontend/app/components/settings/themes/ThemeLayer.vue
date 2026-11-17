<!-- Not Vapor: Vapor delegates events to the document, which can't see into the shadow root. -->
<script setup lang="ts">
/**
 * The parts of personal themes (#88) that a theme must never be able to hide or restyle: the
 * keep-or-revert bar, the safe-mode banner and the theme studio. They render in a shadow root
 * whose host sits in the browser's top layer, so theme selectors can't match them and nothing
 * can be stacked above them. The host's own box is pinned with inline !important declarations,
 * which outrank any stylesheet.
 */
import layerCss from './theme-layer.css?inline'

const themes = useThemesStore()
const appearance = useAppearanceStore()
const { t } = useI18n()
const $t = t

const studioWidth = 360
const hostStyle = [
  'all: initial !important',
  'position: fixed !important',
  'inset: 0 !important',
  'width: 100vw !important',
  'height: 100dvh !important',
  'max-width: none !important',
  'max-height: none !important',
  'margin: 0 !important',
  'padding: 0 !important',
  'border: 0 !important',
  'background: transparent !important',
  'display: block !important',
  'visibility: visible !important',
  'opacity: 1 !important',
  'pointer-events: none !important',
  'overflow: visible !important',
  'transform: none !important',
  'filter: none !important',
  'clip-path: none !important',
  'z-index: 2147483647 !important',
].join(';')

const target = shallowRef<HTMLElement | null>(null)
let host: HTMLElement | null = null
onMounted(() => {
  host = document.createElement('div')
  host.setAttribute('popover', 'manual')
  host.setAttribute('style', hostStyle)
  host.dataset.ircordThemeLayer = ''
  const root = host.attachShadow({ mode: 'open' })
  const style = document.createElement('style')
  style.textContent = layerCss
  const container = document.createElement('div')
  root.append(style, container)
  document.body.appendChild(host)
  target.value = container
  raise()
})
onBeforeUnmount(() => {
  host?.remove()
  delete document.documentElement.dataset.themeStudio
})

/** Shows the host again so it is the newest, topmost entry in the top layer. */
function raise() {
  if (!host?.isConnected) return
  try {
    if (host.matches(':popover-open')) host.hidePopover()
    host.showPopover()
  } catch {
    // No popover support: the fixed host with the highest z-index still covers the page.
  }
}

const banner = computed(() => themes.suspended && !!themes.kept && !themes.draft && !themes.trial)
watch(
  () => [!!themes.trial, banner.value, !!themes.draft],
  () => nextTick(raise),
)
// The studio docks beside the app on wide screens; the app narrows to make room.
watchEffect(() => {
  if (themes.draft) document.documentElement.dataset.themeStudio = ''
  else delete document.documentElement.dataset.themeStudio
})

/** The built-in look, never the theme's: these surfaces must stay readable whatever it does. */
const colors = computed(() => {
  const x = appearance.baseTokens
  return {
    '--l-scheme': appearance.mode,
    '--l-panel': x.panel,
    '--l-raised': x.raised,
    '--l-hover': x.hover,
    '--l-input': x.input,
    '--l-border': x.border,
    '--l-control': x['control-border'],
    '--l-text': x.text,
    '--l-muted': x.muted,
    '--l-accent': x.accent,
    '--l-accent-ink': x['accent-ink'],
    '--l-selection': x.selection,
    '--l-danger': x.danger,
    '--l-note': `color-mix(in srgb, ${x.accent} 12%, ${x.panel})`,
    '--l-warn': `color-mix(in srgb, ${x.yellow} 16%, ${x.panel})`,
    '--studio-width': `${studioWidth}px`,
  }
})
function editKept() {
  if (themes.kept) themes.edit(themes.kept)
}
</script>

<template>
  <Teleport
    v-if="target"
    :to="target"
  >
    <div
      class="layer"
      :style="colors"
    >
      <div
        v-if="themes.trial"
        class="bar"
        role="alertdialog"
        aria-live="assertive"
        :aria-label="$t('themes.trial.title')"
      >
        <p>
          {{ $t('themes.trial.title') }}
          <span class="countdown">{{
            $t('themes.trial.countdown', { seconds: themes.remaining })
          }}</span>
        </p>
        <span class="actions">
          <button
            type="button"
            class="primary"
            @click="themes.keepTrial()"
          >
            {{ $t('themes.trial.keep') }}
          </button>
          <button
            type="button"
            @click="themes.revertTrial()"
          >
            {{ $t('themes.trial.revert') }}
          </button>
        </span>
      </div>
      <div
        v-else-if="banner"
        class="bar"
        role="status"
      >
        <p>{{ themes.safe ? $t('themes.safe.title') : $t('themes.safe.paused') }}</p>
        <span class="actions">
          <button
            type="button"
            class="primary"
            @click="themes.resume()"
          >
            {{ $t('themes.safe.resume') }}
          </button>
          <button
            type="button"
            @click="editKept"
          >
            {{ $t('themes.safe.edit') }}
          </button>
          <button
            type="button"
            @click="themes.useBuiltIn()"
          >
            {{ $t('themes.useBuiltIn') }}
          </button>
        </span>
      </div>
      <ThemeStudio v-if="themes.draft" />
    </div>
  </Teleport>
</template>
