<script setup lang="ts">
import { channelRights, topPrefix } from '../../shared/utils/irc'
import { listInfo, parseChanmodes } from '../../shared/utils/channel-modes'

/** Channel settings frame. Each section owns its forms and permission checks. */
const props = defineProps<{ conversationId: string; initialTab?: string }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const { t } = useI18n()
const translate = (key: string, values?: Record<string, string | number>) => t(key, values ?? {})
const c = computed(() => chat.data?.conversations.find((x) => x.id === props.conversationId))
const network = computed(() => (c.value ? chat.networkOf(c.value) : undefined))
const name = computed(() => `#${c.value?.name ?? ''}`)
const rights = computed(() => channelRights('you', c.value, network.value))
const groups = computed(() => parseChanmodes(network.value?.isupport))
const myPrefix = computed(() => topPrefix('you', c.value, network.value))
const tab = ref(props.initialTab ?? 'overview')
const listLabels = computed(() => listInfo(translate))
const tabs = computed(() => [
  { id: 'overview', label: t('collectionsUi.channel.settings.overview') },
  { id: 'modes', label: t('collectionsUi.channel.settings.modes') },
  { id: 'registration', label: t('collectionsUi.channel.settings.registration') },
  ...[...groups.value.list].map((mode) => ({
    id: `list:${mode}`,
    label: listLabels.value[mode]?.label ?? t('collectionsUi.channel.listModeFallback', { mode }),
  })),
])
const listMode = computed(() => (tab.value.startsWith('list:') ? tab.value.slice(5) : ''))
const rankPhrase = (prefix: string) =>
  t(
    prefix === '~'
      ? 'collectionsUi.channel.settings.owner'
      : prefix === '&'
        ? 'collectionsUi.channel.settings.admin'
        : 'collectionsUi.channel.settings.operator',
  )
</script>
<template>
  <AppDialog
    :title="t('collectionsUi.channel.settings.title', { channel: name })"
    panel
    @close="emit('close')"
  >
    <SettingsLayout
      v-if="c"
      :label="t('collectionsUi.channel.settings.label')"
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
      <div
        class="settings-content [grid-area:content] pt-5 pr-7 pb-7 pl-7 overflow-y-auto max-[700px]:p-4"
      >
        <!-- ChanServ permissions follow accounts, not channel rank. -->
        <p
          v-if="tab !== 'registration'"
          class="rank-note flex items-center gap-2 mb-4.5 py-2.5 px-3 rounded-md bg-card text-[13px]"
          :class="rights.op ? 'can-edit text-success' : 'text-muted-foreground'"
        >
          <AppIcon
            :name="rights.op ? 'shield' : 'lock'"
            :size="15"
          />
          <span v-if="rights.op">{{
            t('collectionsUi.channel.settings.canChangeEverything', { rank: rankPhrase(myPrefix) })
          }}</span>
          <span v-else-if="rights.kick">{{
            t('collectionsUi.channel.settings.canManageLists')
          }}</span>
          <span v-else>{{
            t('collectionsUi.channel.settings.readOnly', { prefix: '@', channel: name })
          }}</span>
        </p>
        <ChannelOverview
          v-if="tab === 'overview'"
          :c="c"
          :network="network"
          :rights="rights"
          @close="emit('close')"
        />
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
          :network-name="network?.name ?? t('collectionsUi.channel.settings.thisNetwork')"
          :op="rights.op"
          :account="network?.account"
        />
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
