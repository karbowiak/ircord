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
  <div class="server-window">
    <ServerConnectionDetails v-if="chat.network" :key="chat.network.id" :network="chat.network" />
    <div class="server-toolbar">
      <label class="server-filter"
        ><AppIcon
          name="search"
          :size="14" /><input
          v-model="filter"
          placeholder="Filter lines"
          aria-label="Filter server lines"
      /></label>
      <span class="server-count">{{ lines.length }} lines</span>
    </div>
    <div
      ref="scroller"
      class="server-log"
      tabindex="0"
      role="log"
      :aria-label="`${chat.network?.name} server messages`"
      @scroll.passive="onScroll"
    >
      <p
        v-if="log.isLoading.value && !lines.length"
        class="server-empty"
      >
        Loading…
      </p>
      <p
        v-else-if="log.error.value"
        class="server-empty"
      >
        {{ errorText(log.error.value) }}
        <button
          class="text-button"
          @click="log.refetch()"
        >
          Try again
        </button>
      </p>
      <div
        v-for="line in lines"
        :key="line.id"
        class="server-line"
        :class="`is-${line.kind}`"
      >
        <time
          :datetime="line.time"
          :title="full(line.time)"
          >{{ time(line.time) }}</time
        ><span
          class="server-command"
          :title="line.source"
          >{{ label(line) }}</span
        ><span class="server-text"><MessageContent :text="line.text" /></span>
      </div>
    </div>
    <button
      v-if="!atBottom"
      class="jump-latest"
      @click="scrollEnd"
    >
      Jump to present<AppIcon
        name="down"
        :size="14"
      />
    </button>
  </div>
</template>
