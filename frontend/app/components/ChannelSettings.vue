<script setup lang="ts">
import { channelRights, topPrefix } from '../../shared/utils/irc'
import { listInfo, parseChanmodes } from '../../shared/utils/channel-modes'

/**
 * Everything IRC lets you see and change about a channel: topic, modes (as advertised by the
 * network's CHANMODES) and its ban/exception/invite/quiet lists. Changes are plain MODE and TOPIC
 * commands; editing is enabled only where your channel rank allows it. Each tab is its own
 * component; this is the frame, the tab list and the note on what your rank allows.
 */
const props = defineProps<{ conversationId: string; initialTab?: string }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const c = computed(() => chat.data?.conversations.find((x) => x.id === props.conversationId))
const network = computed(() => (c.value ? chat.networkOf(c.value) : undefined))
const name = computed(() => `#${c.value?.name ?? ''}`)
const rights = computed(() => channelRights('you', c.value, network.value))
const groups = computed(() => parseChanmodes(network.value?.isupport))
const myPrefix = computed(() => topPrefix('you', c.value, network.value))

const tab = ref(props.initialTab ?? 'overview')
const tabs = computed(() => [
  { id: 'overview', label: 'Overview' },
  { id: 'modes', label: 'Modes' },
  { id: 'registration', label: 'Registration' },
  ...[...groups.value.list].map((mode) => ({
    id: `list:${mode}`,
    label: listInfo[mode]?.label ?? `+${mode} list`,
  })),
])
const listMode = computed(() => (tab.value.startsWith('list:') ? tab.value.slice(5) : ''))

const rankPhrase: Record<string, string> = { '~': 'the owner', '&': 'an admin', '@': 'an operator' }
</script>
<template>
  <AppDialog
    :title="`${name} settings`"
    panel
    @close="emit('close')"
  >
    <SettingsLayout
      v-if="c"
      label="Channel settings"
      class="channel-settings"
    >
      <template #nav>
        <SettingsNavItem
          v-for="item in tabs"
          :key="item.id"
          :active="tab === item.id"
          @click="tab = item.id"
          >{{ item.label }}</SettingsNavItem
        >
      </template>
      <div class="settings-content pt-5 pr-7 pb-7 pl-7 overflow-y-auto">
        <!-- ChanServ permissions follow accounts, not channel rank. -->
        <p
          v-if="tab !== 'registration'"
          class="rank-note flex items-center gap-2 mb-4.5 py-2.5 px-3 rounded-md bg-card text-[13px] text-muted-foreground"
          :class="{ 'can-edit': rights.op }"
        >
          <AppIcon
            :name="rights.op ? 'shield' : 'lock'"
            :size="15"
          />
          <span v-if="rights.op"
            >You’re {{ rankPhrase[myPrefix] ?? 'an operator' }} here and can change everything
            below.</span
          >
          <span v-else-if="rights.kick"
            >As a half-operator you can manage lists and the topic, but not modes.</span
          >
          <span v-else>Read-only: changing these needs operator status (@) in {{ name }}.</span>
        </p>

        <!-- Overview -->
        <ChannelOverview
          v-if="tab === 'overview'"
          :c="c"
          :network="network"
          :rights="rights"
          @close="emit('close')"
        />

        <!-- Modes -->
        <ChannelModes
          v-else-if="tab === 'modes'"
          :c="c"
          :network="network"
          :rights="rights"
        />

        <ChannelRegistration
          v-else-if="tab === 'registration'"
          :conversation-id="conversationId"
          :name="name"
          :network-name="network?.name ?? 'This network'"
          :op="rights.op"
          :account="network?.account"
        />

        <!-- List modes -->
        <ChannelModeList
          v-else
          :key="listMode"
          :c="c"
          :network="network"
          :rights="rights"
          :list-mode="listMode"
        />
      </div>
    </SettingsLayout>
  </AppDialog>
</template>
