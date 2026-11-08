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
    class="about grid gap-3.5 [justify-items:start] max-w-[62ch]"
    data-setting="about-ircord"
  >
    <div class="about-hero flex items-center gap-3.5">
      <button
        ref="logo"
        type="button"
        class="about-logo flex-none w-14 h-14 rounded-[16px] [transition:transform_0.12s]"
        :class="{ pressed: presses > 0 }"
        aria-label="IRCord"
        @click="pressLogo"
      >
        <BrandMark :size="56" />
      </button>
      <div class="about-identity grid gap-0.5">
        <h4 class="about-name m-0 text-[18px] font-[700] tracking-[-0.01em]">IRCord</h4>
        <p
          class="about-version m-0 flex flex-wrap items-center gap-2 text-[13px] text-muted-foreground"
        >
          <template v-if="build.release">Version {{ build.release }}</template>
          <template v-else>Development build</template>
          <span
            v-if="demo"
            class="about-demo-badge py-[1px] px-[7px] rounded-[999px] text-[11px] font-[650] bg-selection text-foreground"
            >Demo</span
          >
        </p>
      </div>
    </div>

    <SettingsDescription>
      IRCord is a persistent, multi-network IRC client for the web. It stays connected to your
      networks while you are away, keeps the history, and speaks plain IRC, using IRCv3 features
      where a network supports them.
    </SettingsDescription>
    <p
      v-if="demo"
      class="settings-description about-demo text-[13px] text-muted-foreground leading-[1.55] py-2.5 px-3 rounded-sm bg-card"
    >
      This is a demo: no IRC network is connected, the people and messages are examples, and Echo
      repeats what you send.
    </p>

    <dl
      class="about-facts grid [grid-template-columns:repeat(auto-fit,_minmax(150px,_1fr))] gap-2 w-full"
    >
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
      class="about-links grid [grid-template-columns:repeat(auto-fit,_minmax(200px,_1fr))] gap-1.5 w-full"
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

    <p class="settings-description about-credits text-muted-foreground text-[12px] leading-[1.55]">
      Built with Nuxt, Vue and Go. Icons by Lucide, emoji data by Emojibase.
    </p>

    <ReleaseHistory />
  </div>
</template>
