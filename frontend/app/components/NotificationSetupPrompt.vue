<script setup lang="ts">
/**
 * The first-use notification offer on phones, tablets and installed apps (#189): one optional
 * banner, never a native prompt on its own. Enable, Repair and Install act only on a click;
 * Not now is remembered for this account on this device, and Settings keeps every option.
 */
import { setupOffer } from '../../shared/notification-setup'

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
    online: online.value,
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

const copy = computed(() =>
  offer.value === 'repair'
    ? {
        title: 'Finish setting up notifications',
        body: 'This device stopped receiving them. Repair it to get mentions and direct messages again.',
        action: 'Repair',
      }
    : offer.value === 'install'
      ? {
          title: 'Want notifications on this device?',
          body: 'On iPhone and iPad they work in the Home Screen app: tap Share (on iOS 26, under ⋯), choose “Add to Home Screen”, then open IRCord from there.',
          action: '',
        }
      : {
          title: 'Get notified on this device?',
          body: 'Mentions and direct messages, even when IRCord is closed. You can change this any time in Settings.',
          action: 'Enable',
        },
)

async function accept() {
  answered.value = true
  const ok = offer.value === 'repair' ? await notifications.repair() : await notifications.enable()
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
    class="setup-prompt"
    aria-labelledby="notification-setup-title"
  >
    <AppIcon
      name="bell"
      :size="18"
    />
    <div class="setup-copy">
      <strong id="notification-setup-title">{{ copy.title }}</strong>
      <span>{{ copy.body }}</span>
      <button
        type="button"
        class="setup-link"
        @click="openSettings"
      >
        Notification settings
      </button>
    </div>
    <div class="setup-actions">
      <button
        v-if="copy.action"
        type="button"
        class="button primary small"
        :disabled="notifications.busy"
        @click="accept"
      >
        {{ copy.action }}
      </button>
      <button
        v-else-if="install.available"
        type="button"
        class="button primary small"
        @click="install.prompt()"
      >
        Install
      </button>
      <button
        type="button"
        class="button secondary small"
        @click="notNow"
      >
        Not now
      </button>
    </div>
  </section>
</template>
<style scoped>
.setup-prompt {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 10px 16px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--muted);
  background: var(--panel);
  border-bottom: 1px solid var(--border);
}
.setup-prompt > svg {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--accent);
}
.setup-copy {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.setup-copy strong {
  color: var(--text);
  font-weight: 550;
}
.setup-link {
  align-self: flex-start;
  font-size: 12px;
  color: var(--muted);
  text-decoration: underline;
  text-underline-offset: 2px;
}
.setup-link:hover {
  color: var(--text);
}
.setup-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  align-self: center;
}
@media (max-width: 520px) {
  .setup-prompt {
    flex-wrap: wrap;
  }
  .setup-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
