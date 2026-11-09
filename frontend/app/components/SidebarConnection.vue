<script setup lang="ts">
import { offlineStatus } from '../utils/offline-archive'
const chat = useChatStore()
const link = useClientLink()
const navigation = useConversationNavigation()
const label = computed(() => {
  if (chat.status === 'offline') return 'Offline'
  if (chat.status === 'syncing') return 'Catching up'
  if (chat.status !== 'connected')
    return chat.status === 'reconnecting' ? 'Reconnecting' : 'Connecting'
  if (link.failing.value) return 'Connection unstable'
  if (chat.network?.status === 'error') return 'IRC connection failed'
  if (chat.network?.status === 'connecting') return 'IRC connecting'
  if (chat.network?.status === 'disconnected') return 'IRC disconnected'
  return 'Connected'
})
const healthy = computed(
  () => chat.status === 'connected' && chat.network?.status === 'connected' && !link.failing.value,
)
const latency = computed(() => {
  const sample = link.samples.value.at(-1)
  return chat.status === 'connected' &&
    !link.failing.value &&
    sample &&
    Date.now() - Date.parse(sample.at) < 75_000
    ? `${Math.round(sample.ms)} ms`
    : ''
})
const detail = computed(() =>
  [
    `${label.value}${latency.value ? ` · ${latency.value} to IRCord` : ''}`,
    offlineStatus.value.phase === 'syncing'
      ? 'Saving history for offline use…'
      : offlineStatus.value.phase === 'unavailable'
        ? 'Offline storage is unavailable.'
        : `${offlineStatus.value.messages} messages and ${offlineStatus.value.files} files saved offline`,
    offlineStatus.value.error,
  ]
    .filter(Boolean)
    .join('\n'),
)
</script>
<template>
  <AppTooltip
    :text="detail"
    side="top"
  >
    <button
      type="button"
      class="sidebar-connection mx-2 mb-2 flex min-h-9 shrink-0 items-center gap-2 rounded-sm px-2 text-[length:var(--ui-caption)] text-muted-foreground hover:bg-accent hover:text-foreground [@media(pointer:coarse)]:min-h-[44px]"
      :aria-label="`Connection details: ${detail}`"
      :disabled="!chat.networkId"
      @click="navigation.open(`server:${chat.networkId}`)"
    >
      <span
        class="size-1.5 shrink-0 rounded-full"
        :class="
          healthy ? 'bg-success' : chat.status === 'offline' ? 'bg-muted-foreground' : 'bg-warning'
        "
        aria-hidden="true"
      />
      <span class="min-w-0 flex-1 truncate text-left">{{ label }}</span>
      <span
        v-if="latency"
        class="tabular-nums"
        >{{ latency }}</span
      >
      <AppIcon
        v-else-if="chat.status === 'offline'"
        name="offline"
        :size="12"
      />
    </button>
  </AppTooltip>
</template>
