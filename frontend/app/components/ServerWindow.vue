<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { ServerLine } from '../../shared/types/chat'
import { stripFormatting } from '../../shared/utils/irc'
import { serverLogQuery } from '../queries/chat'

/** mIRC-style status window: everything the server sends that isn't a conversation. */
const chat = useChatStore()
const { time, full } = useTimeFormat()
const log = useQuery(() => serverLogQuery(chat.serverView))
const filter = ref('')
const scroller = ref<HTMLElement | null>(null)
const atBottom = ref(true)
const lines = computed(() => {
  const items = log.data.value?.items ?? []
  const q = filter.value.trim().toLowerCase()
  return q
    ? items.filter((l) =>
        `${l.command} ${l.source ?? ''} ${stripFormatting(l.text)}`.toLowerCase().includes(q),
      )
    : items
})
/** What precedes the text: the numeric or command, or the sender for notices. */
function label(line: ServerLine) {
  if (line.kind === 'outgoing') return '→'
  if (line.kind === 'notice') return `-${line.source?.split('!')[0] ?? 'server'}-`
  return line.command
}
function onScroll() {
  const el = scroller.value
  if (el) atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40
}
function scrollEnd() {
  nextTick(() => {
    if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
  })
}
watch(
  () => lines.value.length,
  () => atBottom.value && scrollEnd(),
  { flush: 'post' },
)
watch(() => chat.serverView, scrollEnd)
onMounted(scrollEnd)
</script>
<template>
  <div class="server-window relative flex-1 min-h-0 flex flex-col">
    <ServerConnectionDetails
      v-if="chat.network"
      :key="chat.network.id"
      :network="chat.network"
    />
    <div
      class="server-toolbar flex items-center gap-3 py-2 px-4 border-b [border-bottom-style:solid] border-b-border"
    >
      <SearchField
        v-model="filter"
        class="server-filter h-7 w-[min(280px,_60%)]"
        placeholder="Filter lines"
        aria-label="Filter server lines"
      />
      <span class="server-count text-[12px] text-faint">{{ lines.length }} lines</span>
    </div>
    <div
      ref="scroller"
      class="server-log flex-1 min-h-0 overflow-y-auto pt-2 pr-0 pb-4 pl-0 font-code text-[length:calc(var(--message-size)_*_0.92)] leading-[1.55] [touch-action:pan-y_pinch-zoom]"
      tabindex="0"
      role="log"
      :aria-label="`${chat.network?.name} server messages`"
      @scroll.passive="onScroll"
    >
      <p
        v-if="log.isLoading.value && !lines.length"
        class="server-empty py-6 px-4 text-faint font-sans"
      >
        Loading…
      </p>
      <p
        v-else-if="log.error.value"
        class="server-empty py-6 px-4 text-faint font-sans"
      >
        {{ errorText(log.error.value) }}
        <UiButton
          variant="link"
          size="text"
          @click="log.refetch()"
        >
          Try again
        </UiButton>
      </p>
      <div
        v-for="line in lines"
        :key="line.id"
        class="server-line grid [grid-template-columns:auto_9ch_minmax(0,_1fr)] gap-x-3 py-0 px-4 text-body hover:bg-[color-mix(in_srgb,var(--rail)_25%,transparent)] [&.is-error]:text-destructive [&.is-error_.server-command]:text-destructive [&.is-notice_.server-command]:text-warning [&.is-motd_.server-text]:text-muted-foreground [&.is-outgoing_.server-command]:text-primary [&.is-outgoing_.server-text]:text-primary max-[700px]:[grid-template-columns:7ch_minmax(0,_1fr)]"
        :class="`is-${line.kind}`"
      >
        <time
          class="text-faint tabular-nums max-[700px]:hidden"
          :datetime="line.time"
          :title="full(line.time)"
          >{{ time(line.time) }}</time
        ><span
          class="server-command text-faint text-right overflow-hidden text-ellipsis whitespace-nowrap"
          :title="line.source"
          >{{ label(line) }}</span
        ><span class="server-text whitespace-pre-wrap wrap-anywhere"
          ><MessageContent :text="line.text"
        /></span>
      </div>
    </div>
    <button
      v-if="!atBottom"
      class="jump-latest absolute bottom-3 left-1/2 [transform:translateX(-50%)] flex items-center gap-2 bg-primary text-primary-foreground py-1.5 px-3 rounded-[999px] text-[12px] font-[600] shadow-control whitespace-nowrap"
      @click="scrollEnd"
    >
      Jump to present<AppIcon
        name="down"
        :size="14"
      />
    </button>
  </div>
</template>
