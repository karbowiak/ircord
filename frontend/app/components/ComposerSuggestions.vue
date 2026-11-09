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
  groups?: { name: string; description: string }[]
  active: number
  query: string
  listId: string
}>()
const emit = defineEmits<{
  emoji: [item: EmojiItem]
  member: [member: Member]
  group: [group: { name: string; description: string }]
  command: [command: CommandSpec]
  channel: [channel: ChannelSuggestion]
  hover: [index: number]
}>()
const library = useLibraryStore()
const appearance = useAppearanceStore()
const nameStyle = useNameStyle()
const list = ref<HTMLElement | null>(null)
const optionClass =
  'flex items-center gap-2.5 h-8.5 px-2 rounded-sm cursor-pointer text-[14px] text-body [&.active]:bg-accent [&.active]:text-foreground [&_.presence]:border-card [&.active_.presence]:border-accent'
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
  <div
    class="composer-suggestions absolute left-4 right-4 bottom-[calc(100%_-_12px)] z-20 flex flex-col max-h-[min(420px,_50dvh)] pt-2 pr-1.5 pb-1.5 pl-1.5 bg-card rounded-md shadow-popover max-[700px]:left-2 max-[700px]:right-2"
  >
    <p class="suggestions-heading pt-0 pr-2 pb-1.5 pl-2 text-[12px] text-faint">{{ label }}</p>
    <ul
      :id="listId"
      ref="list"
      role="listbox"
      class="m-0 p-0 list-none overflow-y-auto"
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
          :class="[optionClass, { active: index === active }]"
          @mousedown.prevent="emit('command', command)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <span
            class="suggestion-command flex-1 min-w-0 font-code text-[13px] overflow-hidden text-ellipsis whitespace-nowrap"
            >/{{ command.name }}
            <span class="suggestion-args text-faint">{{ command.args }}</span></span
          >
          <span class="suggestion-source text-[12px] text-faint">{{ command.description }}</span>
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
          :class="[optionClass, { active: index === active }]"
          @mousedown.prevent="emit('channel', channel)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <span
            class="suggestion-glyph channel-glyph w-6 flex justify-center text-[20px] leading-[1] text-faint"
            ><AppIcon
              name="hash"
              :size="16"
          /></span>
          <span
            class="suggestion-name flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
            >{{ channel.name
            }}<small
              v-if="channel.topic"
              class="suggestion-topic ml-2.5 text-[12px] text-faint"
              >{{ stripFormatting(channel.topic) }}</small
            ></span
          >
          <span
            v-if="channel.unread"
            class="mention-badge bg-destructive text-rail text-[11px] min-w-4.5 h-4.5 py-0 px-[5px] flex items-center justify-center rounded-[9px] font-[700]"
            >{{ channel.unread }}</span
          >
          <span class="suggestion-code text-[12px] text-faint">{{
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
          :class="[optionClass, { active: index === active }]"
          @mousedown.prevent="emit('member', member)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <UserAvatar
            :member="member"
            size="tiny"
            presence
          />
          <span
            class="suggestion-name flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
            :style="nameStyle(member)"
            >{{ member.displayName }}</span
          >
          <span class="suggestion-code text-[12px] text-faint">@{{ member.nick }}</span>
        </li>
        <li
          v-for="(group, offset) in groups ?? []"
          :id="`${listId}-${members.length + offset}`"
          :key="`group:${group.name}`"
          role="option"
          :aria-selected="members.length + offset === active"
          :data-index="members.length + offset"
          :class="[optionClass, { active: members.length + offset === active }]"
          @mousedown.prevent="emit('group', group)"
          @mousemove="members.length + offset !== active && emit('hover', members.length + offset)"
        >
          <span class="suggestion-glyph w-6 flex justify-center text-faint"
            ><AppIcon
              name="users"
              :size="16"
          /></span>
          <span class="suggestion-name flex-1 min-w-0 truncate">@{{ group.name }}</span>
          <span class="suggestion-source text-[12px] text-faint truncate">{{
            group.description
          }}</span>
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
          :class="[optionClass, { active: index === active }]"
          @mousedown.prevent="emit('emoji', item)"
          @mousemove="index !== active && emit('hover', index)"
        >
          <span class="suggestion-glyph w-6 flex justify-center text-[20px] leading-[1]"
            ><template v-if="item.unicode">{{ emojiText(item, library.library.skinTone) }}</template
            ><NuxtImg
              v-else
              class="size-6 object-contain"
              :src="appearance.animate ? item.url : item.staticUrl || item.url"
              alt=""
              referrerpolicy="no-referrer"
          /></span>
          <span
            class="suggestion-name flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
            >:{{ item.shortcode }}:</span
          >
          <span class="suggestion-source text-[12px] text-faint">{{ source(item) }}</span>
        </li>
      </template>
    </ul>
  </div>
</template>
