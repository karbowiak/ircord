<script setup vapor lang="ts">
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
/** The people in the repository's COLLABORATORS.md, bundled at build time. */
const collaborators: string[] = useAppConfig().collaborators ?? []

const { t } = useI18n()
const $t = t
// Brand, file and licence names are not translated; only the labels around them are.
const links = computed(() => [
  { label: t('about.links.source'), detail: 'GitHub', href: PROJECT_URL },
  {
    label: t('about.links.report'),
    detail: t('mediaUi.about.githubIssues'),
    href: `${PROJECT_URL}/issues`,
  },
  {
    label: t('about.links.releases'),
    detail: t('mediaUi.about.githubReleases'),
    href: `${PROJECT_URL}/releases`,
  },
  { label: t('about.links.license'), detail: 'LICENSE', href: `${PROJECT_URL}/blob/main/LICENSE` },
  // The bundled typefaces' own licenses (scripts/build-font-licenses.mjs).
  { label: t('about.links.fontLicenses'), detail: 'SIL OFL 1.1', href: '/data/font-licenses.txt' },
])

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
        class="about-logo flex-none w-14 h-14 rounded-[16px] [transition:transform_0.12s] focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-primary motion-reduce:transition-none"
        :class="{
          'pressed [transform:rotate(-8deg)_scale(0.96)] motion-reduce:transform-none motion-reduce:opacity-85':
            presses > 0,
        }"
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
          <template v-if="build.release">{{
            $t('about.version', { version: build.release })
          }}</template>
          <template v-else>{{ $t('about.development') }}</template>
          <span
            v-if="demo"
            class="about-demo-badge py-[1px] px-[7px] rounded-[999px] text-[11px] font-[650] bg-selection text-foreground"
            >{{ $t('about.demo') }}</span
          >
        </p>
      </div>
    </div>

    <SettingsDescription>
      {{ $t('about.description') }}
    </SettingsDescription>
    <p
      v-if="demo"
      class="settings-description about-demo text-[13px] text-muted-foreground leading-[1.55] py-2.5 px-3 rounded-sm bg-card"
    >
      {{ $t('about.demoNote') }}
    </p>

    <dl
      class="about-facts grid [grid-template-columns:repeat(auto-fit,_minmax(150px,_1fr))] gap-2 w-full [&>div]:min-w-0 [&>div]:rounded-sm [&>div]:bg-card [&>div]:px-3 [&>div]:py-2.5 [&_code]:font-code [&_code]:text-[12px] [&_dd]:mt-[3px] [&_dd]:text-[13px] [&_dd]:text-foreground [&_dd]:wrap-anywhere [&_dt]:text-[11px] [&_dt]:font-[650] [&_dt]:uppercase [&_dt]:tracking-[0.04em] [&_dt]:text-faint"
    >
      <div>
        <dt>{{ $t('about.build') }}</dt>
        <dd>
          <code>{{ build.id || $t('about.unknown') }}</code>
        </dd>
      </div>
      <div>
        <dt>{{ $t('about.licenseLabel') }}</dt>
        <dd>{{ $t('about.licenseText') }}</dd>
      </div>
    </dl>

    <nav
      class="about-links grid [grid-template-columns:repeat(auto-fit,_minmax(200px,_1fr))] gap-1.5 w-full [&>*]:flex [&>*]:min-w-0 [&>*]:items-center [&>*]:gap-2 [&>*]:rounded-sm [&>*]:px-3 [&>*]:py-2.5 [&>*]:text-start [&>*]:text-[13px] [&>*]:font-[550] [&>*]:text-foreground [&>*]:no-underline [&>*]:shadow-[inset_0_0_0_1px_var(--border)] [&>*:hover]:bg-accent [&_small]:ms-auto [&_small]:text-[12px] [&_small]:font-[450] [&_small]:text-faint [&_svg]:flex-none [&_svg]:text-faint"
      :aria-label="$t('about.linksLabel')"
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
        <span>{{ $t('about.links.releaseNotes') }}</span
        ><small>{{ $t('about.links.below') }}</small
        ><AppIcon
          name="down"
          :size="14"
        />
      </button>
    </nav>

    <section
      v-if="collaborators.length"
      class="about-collaborators grid gap-1.5 w-full"
      aria-labelledby="about-collaborators-title"
    >
      <h4
        id="about-collaborators-title"
        class="text-[11px] font-[650] uppercase tracking-[0.04em] text-faint"
      >
        {{ $t('mediaUi.about.collaborators') }}
      </h4>
      <ul class="flex flex-wrap gap-1.5 m-0 p-0 list-none">
        <li
          v-for="name in collaborators"
          :key="name"
          class="rounded-sm bg-card px-3 py-1.5 text-[13px] text-foreground"
        >
          {{ name }}
        </li>
      </ul>
    </section>

    <p class="settings-description about-credits text-muted-foreground text-[12px] leading-[1.55]">
      {{ $t('about.credits') }}
    </p>

    <ReleaseHistory />
  </div>
</template>
