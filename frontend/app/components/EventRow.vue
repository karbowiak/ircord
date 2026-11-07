<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import { eventDescription } from '../../shared/utils/log'
/** A quiet activity line; consecutive changes can be expanded without avatars. */
const props = defineProps<{ events: Message[] }>()
const emit = defineEmits<{ profile: [id: string] }>()
const chat = useChatStore()
const nameStyle = useNameStyle()
const { time, full } = useTimeFormat()
const expanded = ref(false)
const nick = (id?: string) => chat.member(id ?? '')?.nick ?? id ?? ''
const description = (message: Message) => eventDescription(message, nick)
const single = computed(() => (props.events.length === 1 ? props.events[0]! : null))
const last = computed(() => props.events.at(-1)!)
const summary = computed(
  () =>
    props.events
      .slice(0, 3)
      .map((m) => `${nick(m.event!.actorId)} ${description(m)}`)
      .join(' · ') + (props.events.length > 3 ? ` · ${props.events.length - 3} more` : ''),
)
const icons = {
  join: 'login',
  part: 'logout',
  quit: 'logout',
  nick: 'pencil',
  kick: 'gavel',
  mode: 'shield',
  topic: 'hash',
  invite: 'mail',
} as const
</script>
<template>
  <div class="event-row">
    <AppIcon
      :name="single ? icons[single.event!.type] : 'users'"
      :size="12"
      class="event-icon"
    />
    <span
      v-if="single"
      class="event-text"
      :title="`${nick(single.event!.actorId)} ${description(single)}`"
    >
      <button
        class="event-nick"
        :style="nameStyle(chat.member(single.event!.actorId))"
        @click="emit('profile', single.event!.actorId)"
      >
        {{ nick(single.event!.actorId) }}
      </button>
      {{ ' ' + description(single) }}
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
    <time
      :datetime="last.createdAt"
      :title="full(last.createdAt)"
      >{{ time(last.createdAt) }}</time
    >
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
        {{ ' ' + description(m) }}
      </li>
    </ul>
  </div>
</template>
