<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import { chatterSummary, eventReason, isChatter } from '../../shared/utils/events'
import { eventDescription } from '../../shared/utils/log'
/**
 * A quiet activity line. A run of joins, parts, quits and nick changes reads as one summary
 * that expands to every line; moderation and channel changes stand out on rows of their own.
 */
const props = defineProps<{ events: Message[] }>()
const emit = defineEmits<{ profile: [id: string] }>()
const chat = useChatStore()
const nameStyle = useNameStyle()
const { time, full } = useTimeFormat()
const expanded = ref(false)
const nick = (id?: string) => chat.member(id ?? '')?.nick ?? id ?? ''
const description = (message: Message) => eventDescription(message, nick, false)
const reason = (message: Message) =>
  ['part', 'quit', 'kick'].includes(message.event!.type) ? eventReason(message.event!.text) : ''
const single = computed(() => (props.events.length === 1 ? props.events[0]! : null))
const important = computed(() => !!single.value && !isChatter(single.value))
const last = computed(() => props.events.at(-1)!)
const summary = computed(() => chatterSummary(props.events, nick))
const icons = {
  join: 'login',
  part: 'logout',
  quit: 'logout',
  nick: 'pencil',
  kick: 'gavel',
  mode: 'shield',
  topic: 'hash',
  invite: 'mail',
  registered: 'lock',
  'registration-failed': 'alert',
} as const
</script>
<template>
  <div
    class="event-row"
    :class="{ 'is-important': important }"
  >
    <time
      :datetime="last.createdAt"
      :title="full(last.createdAt)"
      >{{ time(last.createdAt) }}</time
    >
    <AppIcon
      :name="single ? icons[single.event!.type] : 'users'"
      :size="12"
      class="event-icon"
      :class="single && `event-${single.event!.type}`"
    />
    <span
      v-if="single"
      class="event-text"
      :title="important ? undefined : `${nick(single.event!.actorId)} ${description(single)}`"
    >
      <button
        class="event-nick"
        :style="nameStyle(chat.member(single.event!.actorId))"
        @click="emit('profile', single.event!.actorId)"
      >
        {{ nick(single.event!.actorId) }}
      </button>
      {{ ' ' + description(single)
      }}<span
        v-if="reason(single)"
        class="event-reason"
        >— {{ reason(single) }}</span
      >
    </span>
    <button
      v-else
      class="event-text event-summary"
      :aria-expanded="expanded"
      :title="summary"
      @click="expanded = !expanded"
    >
      {{ summary
      }}<AppIcon
        :name="expanded ? 'down' : 'right'"
        :size="12"
      />
    </button>
    <ul
      v-if="expanded && !single"
      class="event-list"
    >
      <li
        v-for="m in events"
        :key="m.id"
        :data-event-id="m.id"
      >
        <time :datetime="m.createdAt">{{ time(m.createdAt) }}</time>
        <button
          class="event-nick"
          :style="nameStyle(chat.member(m.event!.actorId))"
          @click="emit('profile', m.event!.actorId)"
        >
          {{ nick(m.event!.actorId) }}
        </button>
        {{ ' ' + description(m)
        }}<span
          v-if="reason(m)"
          class="event-reason"
          >— {{ reason(m) }}</span
        >
      </li>
    </ul>
  </div>
</template>
