<script setup lang="ts" vapor>
import type { Message } from '../../shared/types/chat'
import { chatterSummary, isChatter } from '../../shared/utils/events'
import { eventDescription, eventPresentation } from '../../shared/utils/log'
/**
 * A quiet activity line. A run of joins, parts, quits and nick changes reads as one summary
 * that expands to every line; moderation and channel changes stand out on rows of their own.
 */
const props = defineProps<{ events: Message[] }>()
const emit = defineEmits<{ profile: [id: string] }>()
const { t, locale } = useI18n()
const chat = useChatStore()
const nameStyle = useNameStyle()
const { time, full } = useTimeFormat()
const expanded = ref(false)
const nick = (id?: string) => chat.member(id ?? '')?.nick ?? id ?? ''
const translateEvent = (key: string, values?: Record<string, string | number>, plural?: number) =>
  values ? (plural === undefined ? t(key, values) : t(key, values, plural)) : t(key)
const description = (message: Message) =>
  eventDescription(message, nick, true, translateEvent, locale.value)
const presentation = (message: Message) =>
  eventPresentation(message, nick, true, translateEvent, locale.value)!
const otherValues = (message: Message) =>
  Object.entries(presentation(message).values).filter(
    ([name]) => name !== 'actor' && name !== 'reason',
  )
const single = computed(() => (props.events.length === 1 ? props.events[0]! : null))
const important = computed(() => !!single.value && !isChatter(single.value))
const last = computed(() => props.events.at(-1)!)
const summary = computed(() => chatterSummary(props.events, nick, translateEvent, locale.value))
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
const colors = {
  join: 'text-success',
  part: 'text-destructive',
  quit: 'text-destructive',
  nick: 'text-faint',
  kick: 'text-destructive',
  mode: 'text-primary',
  topic: 'text-primary',
  invite: 'text-primary',
  registered: 'text-primary',
  'registration-failed': 'text-destructive',
} as const
</script>
<template>
  <div
    class="event-row flex flex-wrap items-center gap-2 pr-12 pl-7 text-[12px] text-muted-foreground hover:bg-[color-mix(in_srgb,var(--rail)_22%,transparent)] [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
    :class="important ? 'is-important py-1' : 'py-0.5'"
  >
    <time
      class="shrink-0 text-[11px] text-faint tabular-nums"
      :class="important && 'self-start mt-px'"
      :datetime="last.createdAt"
      :title="full(last.createdAt)"
      >{{ time(last.createdAt) }}</time
    >
    <AppIcon
      :name="single ? icons[single.event!.type] : 'users'"
      :size="12"
      class="event-icon"
      :class="[
        single ? [`event-${single.event!.type}`, colors[single.event!.type]] : 'text-faint',
        important && 'self-start mt-[3px]',
      ]"
    />
    <span
      v-if="single"
      class="event-text flex-1 min-w-0 text-left overflow-hidden text-ellipsis"
      :class="important ? 'text-body whitespace-normal wrap-anywhere' : 'whitespace-nowrap'"
      :title="important ? undefined : description(single)"
    >
      <i18n-t
        :keypath="presentation(single).key"
        scope="global"
      >
        <template #actor>
          <button
            class="event-nick font-[600] hover:underline"
            :style="nameStyle(chat.member(single.event!.actorId))"
            @click="emit('profile', single.event!.actorId)"
          >
            {{ nick(single.event!.actorId) }}
          </button>
        </template>
        <template #reason>
          <span class="event-reason text-faint">{{ presentation(single).values.reason }}</span>
        </template>
        <template
          v-for="[name, value] in otherValues(single)"
          #[name]
        >
          {{ value }}
        </template>
      </i18n-t>
    </span>
    <button
      v-else
      class="event-text event-summary flex-1 min-w-0 text-left whitespace-nowrap overflow-hidden text-ellipsis inline"
      :aria-expanded="expanded"
      :title="summary"
      @click="expanded = !expanded"
    >
      {{ summary
      }}<AppIcon
        :name="expanded ? 'down' : 'right'"
        :size="12"
        class="align-[-2px] ml-1 text-faint"
      />
    </button>
    <ul
      v-if="expanded && !single"
      class="event-list [flex-basis:100%] list-none mt-0.5 mr-0 mb-1 ml-5.5 p-0 grid gap-0.5 text-[12px]"
    >
      <li
        v-for="m in events"
        :key="m.id"
        :data-event-id="m.id"
      >
        <time
          class="inline-block w-[5ch] mr-2 text-faint tabular-nums"
          :datetime="m.createdAt"
          >{{ time(m.createdAt) }}</time
        >
        <i18n-t
          :keypath="presentation(m).key"
          scope="global"
        >
          <template #actor>
            <button
              class="event-nick font-[600] hover:underline"
              :style="nameStyle(chat.member(m.event!.actorId))"
              @click="emit('profile', m.event!.actorId)"
            >
              {{ nick(m.event!.actorId) }}
            </button>
          </template>
          <template #reason>
            <span class="event-reason text-faint">{{ presentation(m).values.reason }}</span>
          </template>
          <template
            v-for="[name, value] in otherValues(m)"
            #[name]
          >
            {{ value }}
          </template>
        </i18n-t>
      </li>
    </ul>
  </div>
</template>
