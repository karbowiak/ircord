<script setup lang="ts">
import type { Member } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
import type { CommandSpec } from '../composables/useCommands'
import type { ChannelSuggestion } from '../utils/suggestions'
import { stripFormatting } from '../../shared/utils/irc'
const props = defineProps<{
  kind: 'emoji' | 'member' | 'command' | 'channel'
  commands: CommandSpec[]
  channels: ChannelSuggestion[]
  emoji: EmojiItem[]
  members: Member[]
  active: number
  query: string
  listId: string
}>()
const emit = defineEmits<{
  emoji: [item: EmojiItem]
  member: [member: Member]
  command: [command: CommandSpec]
  channel: [channel: ChannelSuggestion]
  hover: [index: number]
}>()
const library = useLibraryStore()
const appearance = useAppearanceStore()
const nameStyle = useNameStyle()
const list = ref<HTMLElement | null>(null)
const label = computed(() =>
  props.kind === 'command'
    ? 'Commands'
    : props.kind === 'channel'
      ? props.query
        ? `Channels matching #${props.query}`
        : 'Channels'
      : props.kind === 'member'
        ? props.query
          ? `Members matching @${props.query}`
          : 'Members'
        : `Emoji matching :${props.query}`,
)
const source = (item: EmojiItem) =>
  item.source === 'workspace' ? 'Server' : item.source === '7tv' ? '7TV' : ''
watch(
  () => props.active,
  (index) =>
    nextTick(() =>
      list.value?.querySelector(`[data-index="${index}"]`)?.scrollIntoView({ block: 'nearest' }),
    ),
)
</script>
<template>
  <div class="composer-suggestions">
    <p class="suggestions-heading">{{ label }}</p>
    <ul
      :id="listId"
      ref="list"
      role="listbox"
      :aria-label="label"
    >
      <template v-if="kind === 'command'">
        <li
          v-for="(command, index) in commands"
          :id="`${listId}-${index}`"
          :key="command.name"
          role="option"
          :aria-selected="index === active"
          :data-index="index"
          :class="{ active: index === active }"
          @mousedown.prevent="emit('command', command)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <span class="suggestion-command"
            >/{{ command.name }} <span class="suggestion-args">{{ command.args }}</span></span
          >
          <span class="suggestion-source">{{ command.description }}</span>
        </li>
      </template>
      <template v-else-if="kind === 'channel'">
        <li
          v-for="(channel, index) in channels"
          :id="`${listId}-${index}`"
          :key="channel.name"
          role="option"
          :aria-selected="index === active"
          :data-index="index"
          :class="{ active: index === active }"
          @mousedown.prevent="emit('channel', channel)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <span class="suggestion-glyph channel-glyph"
            ><AppIcon
              name="hash"
              :size="16"
          /></span>
          <span class="suggestion-name"
            >{{ channel.name
            }}<small
              v-if="channel.topic"
              class="suggestion-topic"
              >{{ stripFormatting(channel.topic) }}</small
            ></span
          >
          <span
            v-if="channel.unread"
            class="mention-badge"
            >{{ channel.unread }}</span
          >
          <span class="suggestion-code">{{
            channel.joined ? 'joined' : `${channel.users?.toLocaleString() ?? '?'} users`
          }}</span>
        </li>
      </template>
      <template v-else-if="kind === 'member'">
        <li
          v-for="(member, index) in members"
          :id="`${listId}-${index}`"
          :key="member.id"
          role="option"
          :aria-selected="index === active"
          :data-index="index"
          :class="{ active: index === active }"
          @mousedown.prevent="emit('member', member)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <UserAvatar
            :member="member"
            size="tiny"
            presence
          />
          <span
            class="suggestion-name"
            :style="nameStyle(member)"
            >{{ member.displayName }}</span
          >
          <span class="suggestion-code">@{{ member.nick }}</span>
        </li>
      </template>
      <template v-else>
        <li
          v-for="(item, index) in emoji"
          :id="`${listId}-${index}`"
          :key="item.id"
          role="option"
          :aria-selected="index === active"
          :data-index="index"
          :class="{ active: index === active }"
          @mousedown.prevent="emit('emoji', item)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <span class="suggestion-glyph"
            ><template v-if="item.unicode">{{ emojiText(item, library.library.skinTone) }}</template
            ><img
              v-else
              :src="appearance.animate ? item.url : item.staticUrl || item.url"
              alt=""
              referrerpolicy="no-referrer"
          /></span>
          <span class="suggestion-name">:{{ item.shortcode }}:</span>
          <span class="suggestion-source">{{ source(item) }}</span>
        </li>
      </template>
    </ul>
  </div>
</template>
