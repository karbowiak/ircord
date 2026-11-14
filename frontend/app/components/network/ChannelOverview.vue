<script setup vapor lang="ts">
import {
  markdownToIrc,
  stripFormatting,
  topPrefix,
  type ChannelRights,
} from '../../../shared/utils/irc'
import type { ChannelSettingsChange, Conversation, Network } from '../../../shared/types/chat'
import { safeHref } from '../../../shared/utils/changelog'

/** Channel settings → Overview: the topic, what everyone is notified about, and leaving. */
const props = defineProps<{ c: Conversation; network?: Network; rights: ChannelRights }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const notifications = useNotificationStore()
const { t, n } = useI18n()
const { name, set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
const date = (iso: string) => new Date(iso).toLocaleDateString(undefined, { dateStyle: 'medium' })
const myPrefix = computed(() => topPrefix('you', props.c, props.network))
// The server supplies the website, so only web links are followed.
const website = computed(() => (props.c.url ? safeHref(props.c.url) : undefined))

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
const levelName = (value: string) =>
  t(
    value === 'all'
      ? 'collectionsUi.channel.overview.all'
      : value === 'mentions'
        ? 'collectionsUi.channel.overview.mentions'
        : 'collectionsUi.channel.overview.mute',
  )
function changeShared(changes: ChannelSettingsChange) {
  const current = shared.value
  if (!current || !canShare.value) return
  // Choosing the value that is already set changes nothing; don't send it.
  if (
    (changes.notifications ?? current.notifications) === current.notifications &&
    (changes.groupMentions ?? current.groupMentions) === current.groupMentions &&
    (changes.nsfw ?? current.nsfw ?? false) === (current.nsfw ?? false)
  )
    return
  return chat.channelSettings.mutateAsync({ c: props.c, changes })
}

const leaveChannel = useLeaveChannel()
async function leave() {
  if (await leaveChannel(props.c)) emit('close')
}
const rankName = (prefix: string) =>
  t(
    prefix === '~'
      ? 'collectionsUi.channel.overview.owner'
      : prefix === '&'
        ? 'collectionsUi.channel.overview.admin'
        : prefix === '@'
          ? 'collectionsUi.channel.overview.operator'
          : prefix === '%'
            ? 'collectionsUi.channel.overview.halfOperator'
            : prefix === '+'
              ? 'collectionsUi.channel.overview.voiced'
              : 'collectionsUi.channel.overview.member',
  )
</script>
<template>
  <div class="appearance-settings grid gap-7">
    <!-- The channel at a glance, before anything that can be changed. -->
    <dl
      class="channel-facts stat-strip"
      :aria-label="t('collectionsUi.channel.overview.about')"
    >
      <div>
        <dt>{{ t('collectionsUi.channel.overview.network') }}</dt>
        <dd>{{ network?.name }}</dd>
      </div>
      <div>
        <dt>{{ t('collectionsUi.channel.overview.people') }}</dt>
        <dd>{{ n(c.memberIds.length) }}</dd>
      </div>
      <div>
        <dt>{{ t('collectionsUi.channel.overview.modes') }}</dt>
        <dd>
          <code>+{{ c.modes || '' }}</code>
        </dd>
      </div>
      <div>
        <dt>{{ t('collectionsUi.channel.overview.yourRank') }}</dt>
        <dd>{{ rankName(myPrefix) }}</dd>
      </div>
      <div v-if="c.createdAt">
        <dt>{{ t('collectionsUi.channel.overview.created') }}</dt>
        <dd>{{ date(c.createdAt) }}</dd>
      </div>
      <div v-if="c.url">
        <dt>{{ t('collectionsUi.channel.overview.website') }}</dt>
        <dd>
          <a
            v-if="website"
            :href="website"
            target="_blank"
            rel="noopener noreferrer"
            >{{ c.url }}</a
          >
          <template v-else>{{ c.url }}</template>
        </dd>
      </div>
    </dl>
    <section class="setting-group grid gap-3.5">
      <h3>{{ t('collectionsUi.channel.overview.topic') }}</h3>
      <form
        class="topic-editor grid gap-2"
        @submit.prevent="saveTopic"
      >
        <UiTextarea
          v-model="topic"
          rows="3"
          :maxlength="maxTopic"
          :readonly="!rights.topic"
          :aria-label="t('collectionsUi.channel.overview.topicLabel')"
          :placeholder="t('collectionsUi.channel.overview.noTopic')"
        />
        <div class="topic-meta flex items-center gap-3 text-[12px] text-faint">
          <span v-if="c.topicSetBy"
            >{{ t('collectionsUi.channel.overview.setBy', { person: c.topicSetBy }) }}
            <template v-if="c.topicSetAt"> · {{ date(c.topicSetAt) }}</template></span
          >
          <span
            v-if="rights.topic"
            class="topic-count tabular-nums"
            >{{ n(topic.length) }}/{{ n(maxTopic) }}</span
          >
          <UiButton
            v-if="rights.topic"
            size="sm"
            class="ml-auto"
            :disabled="!topicChanged"
          >
            {{ t('collectionsUi.channel.overview.saveTopic') }}
          </UiButton>
        </div>
        <FieldHint v-if="!rights.topic && set.has('t')">
          {{ t('collectionsUi.channel.overview.topicLocked') }}
        </FieldHint>
        <FieldHint v-else-if="rights.topic">
          {{ t('collectionsUi.channel.overview.formattingHelp') }}
        </FieldHint>
      </form>
    </section>
    <section
      class="setting-group grid gap-3.5"
      aria-labelledby="channel-wide-heading"
    >
      <h3 id="channel-wide-heading">
        {{ t('collectionsUi.channel.overview.forEveryone', { channel: name }) }}
      </h3>
      <FieldHint class="channel-wide-note">
        <template v-if="!shared">{{ t('collectionsUi.channel.overview.notInChannel') }}</template>
        <template v-else-if="rights.op">{{
          t('collectionsUi.channel.overview.everyoneInChannel', { prefix: '@' })
        }}</template>
        <template v-else>{{
          t('collectionsUi.channel.overview.setByOperators', { prefix: '@' })
        }}</template>
        <template v-if="shared?.updatedBy">
          {{ t('collectionsUi.channel.overview.lastChangedBy', { person: shared.updatedBy })
          }}<template v-if="shared.updatedAt"> · {{ date(shared.updatedAt) }}</template
          >.</template
        >
      </FieldHint>
      <div class="setting-card">
        <div class="setting-row flex items-center justify-between gap-4">
          <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
            ><strong>{{ t('collectionsUi.channel.overview.notifications') }}</strong
            ><small>{{
              t('collectionsUi.channel.overview.notificationsHelp', {
                level: levelName(notifications.settings.channelDefault),
              })
            }}</small></span
          >
          <AppSegmented
            :model-value="shared?.notifications ?? 'default'"
            :label="t('collectionsUi.channel.overview.notificationsForEveryone')"
            :options="
              [
                { value: 'default', label: t('collectionsUi.channel.overview.default') },
                { value: 'all', label: t('collectionsUi.channel.overview.all') },
                { value: 'mentions', label: t('collectionsUi.channel.overview.mentions') },
                { value: 'none', label: t('collectionsUi.channel.overview.mute') },
              ] as const
            "
            :disabled="!canShare"
            reselect
            @change="changeShared({ notifications: $event })"
          />
        </div>
        <SettingRow
          :label="t('collectionsUi.channel.overview.groupMentionsName')"
          :description="t('collectionsUi.channel.overview.groupMentionsDescription')"
        >
          <AppSegmented
            :model-value="shared?.groupMentions ?? 'ops'"
            :label="t('collectionsUi.channel.overview.groupMentionsLabel')"
            :options="
              [
                { value: 'all', label: t('collectionsUi.channel.overview.everyone') },
                { value: 'ops', label: t('collectionsUi.channel.overview.operators') },
                { value: 'off', label: t('collectionsUi.channel.overview.off') },
              ] as const
            "
            :disabled="!canShare"
            reselect
            @change="changeShared({ groupMentions: $event })"
          />
        </SettingRow>
        <SettingRow
          :label="t('collectionsUi.channel.overview.markNsfw')"
          :description="t('collectionsUi.channel.overview.markNsfwHelp')"
        >
          <AppSwitch
            :model-value="shared?.nsfw === true"
            :disabled="!canShare"
            :aria-label="t('collectionsUi.channel.overview.markNsfw')"
            @update:model-value="changeShared({ nsfw: $event })"
          />
        </SettingRow>
      </div>
    </section>
    <section class="setting-group grid gap-3.5">
      <h3>{{ t('collectionsUi.channel.overview.you') }}</h3>
      <div class="setting-card">
        <div class="setting-row flex items-center justify-between gap-4">
          <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
            ><strong>{{ t('collectionsUi.channel.overview.yourNotifications') }}</strong
            ><small>{{ t('collectionsUi.channel.overview.personalHelp') }}</small></span
          >
        </div>
        <div class="setting-row flex items-center justify-between gap-4">
          <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
            ><strong>{{ t('collectionsUi.channel.overview.leaveChannel') }}</strong
            ><small>{{ t('collectionsUi.channel.overview.leaveHelp') }}</small></span
          >
          <UiButton
            variant="destructive"
            size="sm"
            @click="leave"
          >
            {{ t('collectionsUi.channel.overview.leave') }}
          </UiButton>
        </div>
      </div>
    </section>
  </div>
</template>
