<script setup vapor lang="ts">
import {
  markdownToIrc,
  stripFormatting,
  topPrefix,
  type ChannelRights,
} from '../../../shared/utils/irc'
import type { ChannelSettingsChange, Conversation, Network } from '../../../shared/types/chat'

/** Channel settings → Overview: the topic, what everyone is notified about, and leaving. */
const props = defineProps<{ c: Conversation; network?: Network; rights: ChannelRights }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const notifications = useNotificationStore()
const { confirm } = usePrompt()
const { t, n, locale } = useI18n()
const { name, set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
const date = (iso: string) =>
  new Date(iso).toLocaleDateString(locale.value, { dateStyle: 'medium' })
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
    (changes.groupMentions ?? current.groupMentions) === current.groupMentions
  )
    return
  return chat.channelSettings.mutateAsync({ c: props.c, changes })
}

async function leave() {
  if (
    await confirm({
      title: t('collectionsUi.channel.overview.leaveConfirm', { channel: name.value }),
      confirm: t('collectionsUi.channel.overview.leave'),
      danger: true,
    })
  ) {
    chat.leave.mutate({ c: props.c })
    emit('close')
  }
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
  <div class="appearance-settings grid gap-6">
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
    <section class="setting-group grid gap-3.5">
      <h3>{{ t('collectionsUi.channel.overview.about') }}</h3>
      <dl
        class="channel-facts grid [grid-template-columns:repeat(auto-fill,_minmax(180px,_1fr))] gap-[12px_20px] m-0"
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
    </section>
    <section class="setting-group grid gap-3.5">
      <h3>{{ t('collectionsUi.channel.overview.you') }}</h3>
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
    </section>
  </div>
</template>
