<script setup lang="ts">
import { aboutBuild, PROJECT_URL } from '../../shared/utils/about'

/**
 * Settings → About: what IRCord is, which build this page runs, where the source, issues, license
 * and release notes live, and the release history from the changelog (ReleaseHistory).
 *
 * Only public build facts are shown: the release tag the image was built from and Nuxt's build id,
 * both already served to every visitor. Pressing the logo five times opens Hash Field, a small
 * game; the page is complete without it.
 */
const chat = useChatStore()
const config = useRuntimeConfig()
const build = aboutBuild(useAppConfig().release.version, config.app.buildId)
const demo = computed(() => !!chat.data?.demo)

const links = [
  { label: 'Source code', detail: 'GitHub', href: PROJECT_URL },
  { label: 'Report a problem', detail: 'GitHub issues', href: `${PROJECT_URL}/issues` },
  { label: 'All releases', detail: 'GitHub releases', href: `${PROJECT_URL}/releases` },
  { label: 'MIT License', detail: 'LICENSE', href: `${PROJECT_URL}/blob/main/LICENSE` },
  // The bundled typefaces' own licenses (scripts/build-font-licenses.mjs).
  { label: 'Font licenses', detail: 'SIL OFL 1.1', href: '/data/font-licenses.txt' },
]

const PRESSES = 5
const presses = ref(0)
let pressTimer: ReturnType<typeof setTimeout> | undefined
const playing = ref(false)
const logo = ref<HTMLButtonElement | null>(null)

function pressLogo() {
  clearTimeout(pressTimer)
  presses.value += 1
  if (presses.value >= PRESSES) {
    presses.value = 0
    playing.value = true
    return
  }
  // The presses have to come in a row; a pause starts the count again.
  pressTimer = setTimeout(() => (presses.value = 0), 1500)
}

async function exitGame() {
  playing.value = false
  await nextTick()
  logo.value?.focus()
}

function showReleaseNotes() {
  const heading = document.getElementById('release-history-title')
  heading?.scrollIntoView({ block: 'start' })
  heading?.focus({ preventScroll: true })
}

onBeforeUnmount(() => clearTimeout(pressTimer))
</script>
<template>
  <HashField
    v-if="playing"
    @exit="exitGame"
  />
  <div
    v-else
    class="about"
    data-setting="about-ircord"
  >
    <div class="about-hero">
      <button
        ref="logo"
        type="button"
        class="about-logo"
        :class="{ pressed: presses > 0 }"
        aria-label="IRCord"
        @click="pressLogo"
      >
        <BrandMark :size="56" />
      </button>
      <div class="about-identity">
        <h4 class="about-name">IRCord</h4>
        <p class="about-version">
          <template v-if="build.release">Version {{ build.release }}</template>
          <template v-else>Development build</template>
          <span
            v-if="demo"
            class="about-demo-badge"
            >Demo</span
          >
        </p>
      </div>
    </div>

    <p class="settings-description">
      IRCord is a persistent, multi-network IRC client for the web. It stays connected to your
      networks while you are away, keeps the history, and speaks plain IRC, using IRCv3 features
      where a network supports them.
    </p>
    <p
      v-if="demo"
      class="settings-description about-demo"
    >
      This is a demo: no IRC network is connected, the people and messages are examples, and Echo
      repeats what you send.
    </p>

    <dl class="about-facts">
      <div>
        <dt>Build</dt>
        <dd>
          <code>{{ build.id || 'unknown' }}</code>
        </dd>
      </div>
      <div>
        <dt>License</dt>
        <dd>MIT · © 2026 Michael Karbowiak</dd>
      </div>
    </dl>

    <nav
      class="about-links"
      aria-label="IRCord links"
    >
      <a
        v-for="link in links"
        :key="link.href"
        :href="link.href"
        target="_blank"
        rel="noopener noreferrer"
        ><span>{{ link.label }}</span
        ><small>{{ link.detail }}</small
        ><AppIcon
          name="external"
          :size="14"
      /></a>
      <button
        type="button"
        @click="showReleaseNotes"
      >
        <span>Release notes</span><small>Below</small
        ><AppIcon
          name="down"
          :size="14"
        />
      </button>
    </nav>

    <p class="settings-description about-credits">
      Built with Nuxt, Vue and Go. Icons by Lucide, emoji data by Emojibase.
    </p>

    <ReleaseHistory />
  </div>
</template>
