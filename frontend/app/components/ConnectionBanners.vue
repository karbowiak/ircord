<script setup lang="ts">
/**
 * Connection problems: IRCord's own link first, then the open network's IRC connection. A newer
 * IRCord build is announced separately, below them, and only while IRCord is connected.
 */
const chat = useChatStore()
const actions = useWorkspaceActions()
const update = useAppUpdate()
const changelogOpen = ref(false)
/** The open channel you're not in. Also true for a moment while a reconnect rejoins channels. */
const notJoined = computed(
  () => !chat.data?.demo && chat.active?.kind === 'channel' && !chat.active.joined,
)
/** Who kicked you from the open channel. IRCord doesn't rejoin it until you do. */
const kicked = computed(() => {
  const kick = notJoined.value ? chat.active?.kicked : undefined
  if (!kick) return undefined
  return { by: chat.member(kick.byId)?.nick ?? 'someone', reason: kick.reason?.trim() }
})
// The banners mount with their text, which screen readers often skip, so changes announce through
// the persistent announcer once they settle. Switching networks or conversations is not news; a
// connection problem that clears is.
const ircordProblem = computed(() =>
  chat.status === 'connected'
    ? ''
    : chat.status === 'offline'
      ? 'You’re offline. Showing what was loaded; drafts are kept.'
      : chat.status === 'syncing'
        ? 'Catching up…'
        : chat.status === 'connecting'
          ? 'Connecting…'
          : 'Reconnecting…',
)
const networkProblem = computed(() => {
  const network = chat.network
  if (!network || network.status === 'connected') return ''
  return network.status === 'connecting'
    ? `${network.name} is connecting…`
    : network.status === 'error'
      ? `${network.name} couldn’t connect: ${network.statusDetail ?? 'unknown error'}`
      : `${network.name} is disconnected.`
})
const connectionText = computed(() => ircordProblem.value || networkProblem.value)
const banners = useStatusAnnouncer(1000)
let hadProblem = !!connectionText.value
watch([connectionText, () => chat.networkId], ([text, networkId], [, previousNetwork]) => {
  const problem = hadProblem
  hadProblem = !!text
  if (networkId !== previousNetwork) return banners.cancel()
  if (text) banners.later(text, chat.network?.status === 'error' ? 'assertive' : 'polite')
  else if (problem) banners.later('Connected.')
})
useAnnouncedStatus(
  () =>
    notJoined.value && chat.active
      ? kicked.value
        ? `You were kicked from #${chat.active.name} by ${kicked.value.by}${kicked.value.reason ? ` (${kicked.value.reason})` : ''}.`
        : `You’re not in #${chat.active.name}.`
      : '',
  {
    context: () => chat.activeId,
  },
)
useAnnouncedStatus(() =>
  chat.status === 'connected' && update.available
    ? 'IRCord was updated. Reload to get the new version.'
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
  <!-- Connection state comes and goes (a reconnect also rejoins channels), so these banners
       overlay the timeline instead of pushing it down. -->
  <div
    v-if="
      chat.status !== 'connected' ||
      (chat.network && chat.network.status !== 'connected') ||
      notJoined
    "
    class="connection-overlay absolute top-[var(--header-height)] left-0 right-0 z-4 shadow-control"
  >
    <div
      v-if="chat.status !== 'connected'"
      class="connection-banner flex items-center gap-2 text-warning [background:color-mix(in_srgb,_var(--yellow)_10%,_var(--main))] py-1.5 px-4 text-[13px] shrink-0"
    >
      <AppIcon
        :name="chat.status === 'offline' ? 'offline' : 'refresh'"
        :size="14"
      /><span>{{ ircordProblem }}</span
      ><UiButton
        variant="ghost"
        size="xs"
        class="text-foreground first-of-type:ms-auto"
        @click="chat.reconnect()"
        >Retry now</UiButton
      >
    </div>
    <div
      v-else-if="chat.network && chat.network.status !== 'connected'"
      class="connection-banner flex items-center gap-2 text-warning [background:color-mix(in_srgb,_var(--yellow)_10%,_var(--main))] py-1.5 px-4 text-[13px] shrink-0 [&.is-error]:text-destructive [&.is-error]:bg-[color-mix(in_srgb,var(--danger)_10%,var(--main))] [&.is-disconnected]:text-muted-foreground [&.is-disconnected]:bg-card"
      :class="`is-${chat.network.status}`"
    >
      <AppIcon
        :name="chat.network.status === 'connecting' ? 'refresh' : 'offline'"
        :size="14"
      /><span
        ><strong>{{ chat.network.name }}</strong>
        {{
          chat.network.status === 'connecting'
            ? 'is connecting…'
            : chat.network.status === 'error'
              ? `couldn’t connect: ${chat.network.statusDetail ?? 'unknown error'}`
              : 'is disconnected.'
        }}</span
      >
      <UiButton
        variant="ghost"
        size="xs"
        class="text-foreground first-of-type:ms-auto"
        @click="actions.editNetwork(chat.network.id)"
      >
        Edit network
      </UiButton>
      <UiButton
        variant="ghost"
        size="xs"
        class="text-foreground first-of-type:ms-auto"
        @click="chat.connection.mutate(chat.network)"
      >
        {{
          chat.network.status === 'connecting'
            ? 'Cancel connection'
            : chat.network.status === 'error'
              ? 'Retry now'
              : 'Connect'
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
      /><span
        v-if="kicked"
        class="kicked-notice"
        >You were kicked from #{{ chat.active.name }} by <strong>{{ kicked.by }}</strong
        >{{ kicked.reason ? ` (${kicked.reason})` : '' }}.</span
      ><span v-else>You’re not in #{{ chat.active.name }}.</span>
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
        {{ kicked ? 'Rejoin' : 'Join channel' }}
      </UiButton>
    </div>
  </div>
  <div
    v-if="chat.status === 'connected' && update.available"
    class="connection-banner flex items-center gap-2 text-primary bg-[color-mix(in_srgb,var(--accent)_10%,var(--main))] py-1.5 px-4 text-[13px] shrink-0 is-update"
  >
    <AppIcon
      name="refresh"
      :size="14"
    /><span>IRCord was updated. Reload to get the new version.</span>
    <UiButton
      variant="ghost"
      size="xs"
      class="text-foreground first-of-type:ms-auto"
      aria-haspopup="dialog"
      @click="changelogOpen = true"
    >
      What’s new
    </UiButton>
    <UiButton
      variant="ghost"
      size="xs"
      class="text-foreground first-of-type:ms-auto"
      @click="update.reload()"
    >
      Reload now
    </UiButton>
    <UiButton
      variant="ghost"
      size="xs"
      class="text-foreground first-of-type:ms-auto"
      @click="update.dismiss()"
    >
      Later
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
