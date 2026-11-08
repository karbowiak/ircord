<script setup lang="ts">
import {
  markdownToIrc,
  stripFormatting,
  topPrefix,
  type ChannelRights,
} from '../../shared/utils/irc'
import type { ChannelSettingsChange, Conversation, Network } from '../../shared/types/chat'

/** Channel settings → Overview: the topic, what everyone is notified about, and leaving. */
const props = defineProps<{ c: Conversation; network?: Network; rights: ChannelRights }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const notifications = useNotificationStore()
const { confirm } = usePrompt()
const { name, set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
const date = channelDate
const myPrefix = computed(() => topPrefix('you', props.c, props.network))

const topic = ref(stripFormatting(props.c.topic ?? ''))
const topicChanged = computed(() => topic.value !== stripFormatting(props.c.topic ?? ''))
watch(
  () => props.c.topic,
  (value) => (topic.value = stripFormatting(value ?? '')),
)
const maxTopic = computed(() => Number(props.network?.isupport.TOPICLEN ?? 390))
function saveTopic() {
  send(`TOPIC ${name.value} :${markdownToIrc(topic.value.trim())}`)
}

// ---- Channel-wide settings -----------------------------------------------------------------
const shared = computed(() => props.c.channelSettings)
const canShare = computed(() => !!shared.value && props.rights.op)
const levelName: Record<string, string> = { all: 'All', mentions: 'Mentions', none: 'Mute' }
function changeShared(changes: ChannelSettingsChange) {
  const current = shared.value
  if (!current || !canShare.value) return
  // Choosing the value that is already set changes nothing; don't send it.
  if ((changes.notifications ?? current.notifications) === current.notifications) return
  return chat.channelSettings.mutateAsync({ c: props.c, changes })
}

async function leave() {
  if (await confirm({ title: `Leave ${name.value}?`, confirm: 'Leave', danger: true })) {
    chat.leave.mutate({ c: props.c })
    emit('close')
  }
}
const rankName: Record<string, string> = {
  '~': 'Owner',
  '&': 'Admin',
  '@': 'Operator',
  '%': 'Half-operator',
  '+': 'Voiced',
}
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <section class="setting-group grid gap-3.5">
      <h3>Topic</h3>
      <form
        class="topic-editor grid gap-2"
        @submit.prevent="saveTopic"
      >
        <UiTextarea
          v-model="topic"
          rows="3"
          :maxlength="maxTopic"
          :readonly="!rights.topic"
          aria-label="Channel topic"
          placeholder="No topic set"
        />
        <div class="topic-meta flex items-center gap-3 text-[12px] text-faint">
          <span v-if="c.topicSetBy"
            >Set by <strong>{{ c.topicSetBy }}</strong>
            <template v-if="c.topicSetAt"> · {{ date(c.topicSetAt) }}</template></span
          >
          <span
            v-if="rights.topic"
            class="topic-count tabular-nums"
            >{{ topic.length }}/{{ maxTopic }}</span
          >
          <UiButton
            size="sm"
            v-if="rights.topic"
            :disabled="!topicChanged"
          >
            Save topic
          </UiButton>
        </div>
        <FieldHint v-if="!rights.topic && set.has('t')">
          The topic is locked (+t): only operators can change it.
        </FieldHint>
        <FieldHint v-else-if="rights.topic">
          **bold**, *italic* and __underline__ are sent as IRC formatting.
        </FieldHint>
      </form>
    </section>
    <section class="setting-group grid gap-3.5">
      <h3>About</h3>
      <dl
        class="channel-facts grid [grid-template-columns:repeat(auto-fill,_minmax(180px,_1fr))] gap-[12px_20px] m-0"
      >
        <div>
          <dt>Network</dt>
          <dd>{{ network?.name }}</dd>
        </div>
        <div>
          <dt>People</dt>
          <dd>{{ c.memberIds.length }}</dd>
        </div>
        <div>
          <dt>Modes</dt>
          <dd>
            <code>+{{ c.modes || '' }}</code>
          </dd>
        </div>
        <div>
          <dt>Your rank</dt>
          <dd>{{ rankName[myPrefix] ?? 'Member' }}</dd>
        </div>
        <div v-if="c.createdAt">
          <dt>Created</dt>
          <dd>{{ date(c.createdAt) }}</dd>
        </div>
        <div v-if="c.url">
          <dt>Website</dt>
          <dd>
            <a
              :href="c.url"
              target="_blank"
              rel="noopener noreferrer"
              >{{ c.url }}</a
            >
          </dd>
        </div>
      </dl>
    </section>
    <section
      class="setting-group grid gap-3.5"
      aria-labelledby="channel-wide-heading"
    >
      <h3 id="channel-wide-heading">For everyone in {{ name }}</h3>
      <FieldHint class="channel-wide-note">
        <template v-if="!shared"
          >These channel-wide settings show once you’re in the channel and connected.</template
        >
        <template v-else-if="rights.op"
          >These apply to every IRCord member of this channel. Operators (@) and higher can change
          them.</template
        >
        <template v-else
          >Set by the channel’s operators for everyone here. Changing them needs operator status
          (@).</template
        >
        <template v-if="shared?.updatedBy">
          Last changed by <strong>{{ shared.updatedBy }}</strong
          ><template v-if="shared.updatedAt"> · {{ date(shared.updatedAt) }}</template
          >.</template
        >
      </FieldHint>
      <div class="setting-row flex items-center justify-between gap-4">
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong>Notifications</strong
          ><small
            >Default uses each member’s own channel default (yours:
            {{ levelName[notifications.settings.channelDefault] }}). Mute silences the channel,
            mentions included.</small
          ></span
        >
        <AppSegmented
          :model-value="shared?.notifications ?? 'default'"
          label="Notifications for everyone in this channel"
          :options="
            [
              { value: 'default', label: 'Default' },
              { value: 'all', label: 'All' },
              { value: 'mentions', label: 'Mentions' },
              { value: 'none', label: 'Mute' },
            ] as const
          "
          :disabled="!canShare"
          reselect
          @change="changeShared({ notifications: $event })"
        />
      </div>
    </section>
    <section class="setting-group grid gap-3.5">
      <h3>You</h3>
      <div class="setting-row flex items-center justify-between gap-4">
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong>Your notifications</strong
          ><small
            >This device’s notification permission, quiet hours, highlight words, the ignore list
            and your own channel default stay personal. Find them in User settings →
            Notifications.</small
          ></span
        >
      </div>
      <div class="setting-row flex items-center justify-between gap-4">
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong>Leave channel</strong><small>You can join again any time.</small></span
        >
        <UiButton
          variant="destructive"
          size="sm"
          @click="leave"
        >
          Leave
        </UiButton>
      </div>
    </section>
  </div>
</template>
