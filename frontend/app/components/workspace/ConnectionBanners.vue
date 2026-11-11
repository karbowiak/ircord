<script setup lang="ts" vapor>
import { computed, ref, watch } from 'vue'
/**
 * Connection problems: IRCord's own link first, then the open network's IRC connection. A newer
 * IRCord build is announced separately, below them, and only while IRCord is connected.
 */
const props = withDefaults(defineProps<{ placement?: 'timeline' | 'composer' }>(), {
  placement: 'timeline',
})
const atComposer = computed(() => props.placement === 'composer')
const chat = useChatStore()
const { t } = useI18n()
const $t = t
const actions = useWorkspaceActions()
const update = useAppUpdate()
const changelogOpen = ref(false)
useViewHistory(`connection-changelog:${props.placement}`, changelogOpen)
/** The open channel you're not in. Also true for a moment while a reconnect rejoins channels. */
const notJoined = computed(
  () =>
    !atComposer.value &&
    !chat.data?.demo &&
    chat.active?.kind === 'channel' &&
    !chat.active.party &&
    !chat.active.joined,
)
/** Who kicked you from the open channel. IRCord doesn't rejoin it until you do. */
const kicked = computed(() => {
  const kick = notJoined.value ? chat.active?.kicked : undefined
  if (!kick) return undefined
  return { by: chat.member(kick.byId)?.nick ?? t('timeline.someone'), reason: kick.reason?.trim() }
})
// The banners mount with their text, which screen readers often skip, so changes announce through
// the persistent announcer once they settle. Switching networks or conversations is not news; a
// connection problem that clears is.
const ircordProblem = computed(() =>
  !atComposer.value || chat.status === 'connected'
    ? ''
    : chat.status === 'offline'
      ? t('connection.offline')
      : chat.recoveryFailed
        ? t('connection.failed')
        : chat.status === 'syncing'
          ? t('connection.syncing')
          : chat.status === 'connecting'
            ? t('connection.connecting')
            : t('connection.reconnecting'),
)
const networkProblem = computed(() => {
  const network = chat.network
  if (atComposer.value || chat.status !== 'connected' || !network || network.status === 'connected')
    return ''
  return network.status === 'connecting'
    ? t('connection.networkConnecting', { name: network.name })
    : network.status === 'error'
      ? t('connection.networkError', {
          name: network.name,
          error: network.statusDetail ?? t('connection.unknownError'),
        })
      : t('connection.networkDisconnected', { name: network.name })
})
const historyText = computed(() =>
  atComposer.value && chat.status === 'connected' && chat.historyProblem
    ? t(chat.historyRefreshing ? 'connection.historyRefreshing' : 'connection.historyFailed')
    : '',
)
const connectionText = computed(
  () => ircordProblem.value || networkProblem.value || historyText.value,
)
const banners = useStatusAnnouncer(1000)
let hadProblem = !!connectionText.value
watch([connectionText, () => chat.networkId], ([text, networkId], [, previousNetwork]) => {
  const problem = hadProblem
  hadProblem = !!text
  if (networkId !== previousNetwork) return banners.cancel()
  if (text) banners.later(text, chat.network?.status === 'error' ? 'assertive' : 'polite')
  else if (problem) banners.later(t('connection.connected'))
})
useAnnouncedStatus(
  () =>
    notJoined.value && chat.active
      ? kicked.value
        ? t(kicked.value.reason ? 'connection.kickedWithReason' : 'connection.kicked', {
            channel: `#${chat.active.name}`,
            by: kicked.value.by,
            reason: kicked.value.reason ?? '',
          })
        : t('connection.notJoined', { channel: `#${chat.active.name}` })
      : '',
  {
    context: () => chat.activeId,
  },
)
useAnnouncedStatus(() =>
  !atComposer.value && chat.status === 'connected' && update.available
    ? t('connection.updated')
    : '',
)
// A rollback to this page's build clears the notice; a later update must not reopen the dialog.
watch(
  () => update.latestId,
  (id) => {
    if (!id) changelogOpen.value = false
  },
)
</script>
<template>
  <!-- The positioned message-input wrapper is the containing block. This lane ends exactly at
       the composer's top and never changes the timeline's height or reading position. -->
  <div
    v-if="atComposer && (chat.status !== 'connected' || chat.historyProblem)"
    class="recovery-lane absolute inset-x-0 bottom-full z-4 flex min-h-10 items-center gap-2 bg-background px-3 py-1.5 text-[13px] text-muted-foreground shadow-control [@media(pointer:coarse)]:min-h-11"
    :class="{
      'text-warning [background:color-mix(in_srgb,_var(--yellow)_10%,_var(--main))]':
        (chat.recoveryFailed && chat.status !== 'offline') ||
        (chat.status === 'connected' && chat.historyProblem),
    }"
  >
    <AppIcon
      :name="chat.status === 'offline' ? 'offline' : 'refresh'"
      :size="14"
      class="shrink-0"
    />
    <span class="min-w-0 flex-1 [overflow-wrap:anywhere]">{{ ircordProblem || historyText }}</span>
    <UiButton
      v-if="chat.status !== 'connected' && chat.status !== 'offline' && chat.recoveryFailed"
      variant="ghost"
      size="xs"
      class="shrink-0 text-foreground [@media(pointer:coarse)]:min-h-11"
      @click="chat.reconnect()"
      >{{ $t('connection.retry') }}</UiButton
    >
    <UiButton
      v-else-if="chat.status === 'connected' && chat.historyProblem"
      variant="ghost"
      size="xs"
      class="shrink-0 text-foreground [@media(pointer:coarse)]:min-h-11"
      :disabled="chat.historyRefreshing"
      @click="chat.refreshHistory()"
      >{{ $t('connection.refreshMessages') }}</UiButton
    >
  </div>
  <!-- Connection state comes and goes (a reconnect also rejoins channels), so these banners
       overlay the timeline instead of pushing it down. -->
  <div
    v-if="
      !atComposer &&
      chat.status === 'connected' &&
      ((chat.network && chat.network.status !== 'connected') || notJoined)
    "
    class="connection-overlay absolute top-[var(--header-height)] left-0 right-0 z-4 shadow-control"
  >
    <div
      v-if="chat.network && chat.network.status !== 'connected'"
      class="connection-banner flex items-center gap-2 text-warning [background:color-mix(in_srgb,_var(--yellow)_10%,_var(--main))] py-1.5 px-4 text-[13px] shrink-0 [&.is-error]:text-destructive [&.is-error]:bg-[color-mix(in_srgb,var(--danger)_10%,var(--main))] [&.is-disconnected]:text-muted-foreground [&.is-disconnected]:bg-card"
      :class="`is-${chat.network.status}`"
    >
      <AppIcon
        :name="chat.network.status === 'connecting' ? 'refresh' : 'offline'"
        :size="14"
      /><i18n-t
        tag="span"
        :keypath="
          chat.network.status === 'connecting'
            ? 'connection.networkConnecting'
            : chat.network.status === 'error'
              ? 'connection.networkError'
              : 'connection.networkDisconnected'
        "
        ><template #name
          ><strong>{{ chat.network.name }}</strong></template
        ><template #error>{{
          chat.network.statusDetail ?? $t('connection.unknownError')
        }}</template></i18n-t
      >
      <UiButton
        variant="ghost"
        size="xs"
        class="text-foreground first-of-type:ms-auto"
        @click="actions.editNetwork(chat.network.id)"
      >
        {{ $t('connection.editNetwork') }}
      </UiButton>
      <UiButton
        variant="ghost"
        size="xs"
        class="text-foreground first-of-type:ms-auto"
        @click="chat.connection.mutate(chat.network)"
      >
        {{
          chat.network.status === 'connecting'
            ? $t('connection.cancelConnection')
            : chat.network.status === 'error'
              ? $t('connection.retry')
              : $t('networkForm.connect')
        }}
      </UiButton>
    </div>
    <div
      v-else-if="notJoined && chat.active"
      class="connection-banner flex items-center gap-2 text-warning [background:color-mix(in_srgb,_var(--yellow)_10%,_var(--main))] py-1.5 px-4 text-[13px] shrink-0"
    >
      <AppIcon
        :name="kicked ? 'gavel' : 'hash'"
        :size="14"
      /><i18n-t
        v-if="kicked"
        class="kicked-notice"
        tag="span"
        :keypath="kicked.reason ? 'connection.kickedWithReason' : 'connection.kicked'"
        ><template #channel>{{ `#${chat.active.name}` }}</template
        ><template #by
          ><strong>{{ kicked.by }}</strong></template
        ><template #reason>{{ kicked.reason }}</template></i18n-t
      ><span v-else>{{ $t('connection.notJoined', { channel: `#${chat.active.name}` }) }}</span>
      <UiButton
        variant="ghost"
        size="xs"
        class="text-foreground first-of-type:ms-auto"
        :disabled="chat.join.isLoading"
        @click="
          chat.join.mutate({
            networkId: chat.active.networkId,
            name: chat.active.target ?? `#${chat.active.name}`,
          })
        "
      >
        {{ kicked ? $t('connection.rejoin') : $t('connection.joinChannel') }}
      </UiButton>
    </div>
  </div>
  <div
    v-if="!atComposer && chat.status === 'connected' && update.available"
    class="connection-banner flex items-center gap-2 text-primary bg-[color-mix(in_srgb,var(--accent)_10%,var(--main))] py-1.5 px-4 text-[13px] shrink-0 is-update"
  >
    <AppIcon
      name="refresh"
      :size="14"
    /><span>{{ $t('connection.updated') }}</span>
    <UiButton
      variant="ghost"
      size="xs"
      class="text-foreground first-of-type:ms-auto"
      aria-haspopup="dialog"
      @click="changelogOpen = true"
    >
      {{ $t('connection.whatsNew') }}
    </UiButton>
    <UiButton
      variant="ghost"
      size="xs"
      class="text-foreground first-of-type:ms-auto"
      @click="update.reload()"
    >
      {{ $t('common.reloadNow') }}
    </UiButton>
    <UiButton
      variant="ghost"
      size="xs"
      class="text-foreground first-of-type:ms-auto"
      @click="update.dismiss()"
    >
      {{ $t('connection.later') }}
    </UiButton>
  </div>
  <!-- Outside the banner, so a reconnect that hides the banner leaves the open dialog alone. -->
  <ChangelogDialog
    v-if="changelogOpen && update.latestId"
    :announced="update.latestId"
    @close="changelogOpen = false"
    @reload="update.reload()"
  />
</template>
