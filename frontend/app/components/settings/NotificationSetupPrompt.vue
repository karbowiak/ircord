<script setup vapor lang="ts">
/**
 * The first-use notification offer on phones, tablets and installed apps (#189): one optional
 * banner, never a native prompt on its own. Enable, Repair and Install act only on a click;
 * Not now is remembered for this account on this device, and Settings keeps every option.
 */
import { setupOffer } from '../../../shared/notification-setup'

const props = defineProps<{
  /** Something else has the user's attention (a dialog, a drawer); wait until it is gone. */
  busy: boolean
}>()
const notifications = useNotificationStore()
const install = useInstallPrompt()
const actions = useWorkspaceActions()

const media = (query: string) => window.matchMedia?.(query).matches ?? false
/** A phone or tablet (touch first), or IRCord running as an installed app. */
const eligible = computed(() => notifications.installed || media('(pointer: coarse)'))
const online = ref(navigator.onLine)
const setOnline = () => (online.value = navigator.onLine)
window.addEventListener('online', setOnline)
window.addEventListener('offline', setOnline)
onBeforeUnmount(() => {
  window.removeEventListener('online', setOnline)
  window.removeEventListener('offline', setOnline)
})

const offer = computed(() =>
  setupOffer({
    facts: notifications.facts,
    eligible: eligible.value,
    chosen: notifications.chosen,
    record: notifications.setupRecord,
    online: online.value && !!notifications.currentDevice?.notifications?.initialized,
  }),
)

/**
 * Appears once the workspace has settled, and not while the user is typing or another dialog
 * or drawer is open. Once shown, it stays until answered (or until there is nothing to offer).
 */
const shown = ref(false)
/** Answered in this page: hide even when the offer would come back (e.g. a failed setup). */
const answered = ref(false)
const typing = () => {
  const el = document.activeElement as HTMLElement | null
  return !!el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName))
}
let timer: ReturnType<typeof setTimeout> | undefined
function consider() {
  clearTimeout(timer)
  if (shown.value || answered.value || !offer.value || props.busy) return
  timer = setTimeout(() => {
    if (!offer.value || props.busy || answered.value) return
    if (typing()) return consider()
    shown.value = true
  }, 1500)
}
watch([offer, () => props.busy], consider, { immediate: true })
onBeforeUnmount(() => clearTimeout(timer))
const visible = computed(() => shown.value && !answered.value && !!offer.value && !props.busy)

const { t } = useI18n()
const $t = t
const copy = computed(() =>
  offer.value === 'repair'
    ? {
        title: t('notifications.setupPrompt.repair.title'),
        body: t('notifications.setupPrompt.repair.body'),
        action:
          notifications.permission === 'granted'
            ? t('notifications.setupPrompt.repair.action')
            : t('notifications.setupPrompt.enable.action'),
      }
    : offer.value === 'install'
      ? {
          title: t('notifications.setupPrompt.install.title'),
          body: t('notifications.setupPrompt.install.body'),
          action: '',
        }
      : {
          title: t('notifications.setupPrompt.enable.title'),
          body: t('notifications.setupPrompt.enable.body'),
          action: t('notifications.setupPrompt.enable.action'),
        },
)

async function accept() {
  answered.value = true
  const ok =
    offer.value === 'repair' && notifications.permission === 'granted'
      ? await notifications.repair()
      : await notifications.enable()
  // The native prompt was dismissed (or setup failed before permission): don't offer again on
  // the next load, where repeated prompts lead browsers to block the site. Settings still offers it.
  if (!ok && notifications.permission !== 'granted') notifications.rememberSetup('not-now')
}
function notNow() {
  answered.value = true
  notifications.rememberSetup('not-now')
}
/** Setting up from Settings instead counts as a deferral: Settings is where it stays. */
function openSettings() {
  notNow()
  actions.settings('notifications')
}
</script>
<template>
  <section
    v-if="visible"
    class="setup-prompt flex max-[520px]:flex-wrap items-start gap-3 px-4 py-2.5 shrink-0 text-[13px] text-muted-foreground bg-card border-b [border-bottom-style:solid] border-b-border"
    aria-labelledby="notification-setup-title"
  >
    <AppIcon
      name="bell"
      :size="18"
      class="shrink-0 mt-0.5 text-primary"
    />
    <div class="setup-copy flex flex-1 min-w-0 flex-col gap-0.5">
      <strong
        id="notification-setup-title"
        class="text-foreground font-[550]"
        >{{ copy.title }}</strong
      >
      <span>{{ copy.body }}</span>
      <UiButton
        variant="inline"
        size="inline"
        type="button"
        class="setup-link self-start text-[12px] text-muted-foreground underline hover:not-disabled:text-foreground"
        @click="openSettings"
      >
        {{ $t('notifications.setupPrompt.settingsLink') }}
      </UiButton>
    </div>
    <div
      class="setup-actions flex gap-2 shrink-0 self-center max-[520px]:w-full max-[520px]:justify-end"
    >
      <UiButton
        v-if="copy.action"
        size="sm"
        type="button"
        :disabled="notifications.busy"
        @click="accept"
      >
        {{ copy.action }}
      </UiButton>
      <UiButton
        v-else-if="install.available"
        size="sm"
        type="button"
        @click="install.prompt()"
      >
        {{ $t('common.install') }}
      </UiButton>
      <UiButton
        variant="secondary"
        size="sm"
        type="button"
        @click="notNow"
      >
        {{ $t('common.notNow') }}
      </UiButton>
    </div>
  </section>
</template>
