<script setup lang="ts" vapor>
import { useQuery } from '@pinia/colada'
import type { ServerLine } from '../../../shared/types/chat'
import { stripFormatting } from '../../../shared/utils/irc'
import { groupServerLog, type ServerLogItem } from '../../../shared/utils/server-log'
import { connectionQuery, serverLogQuery } from '../../queries/chat'

/**
 * mIRC-style status window: summary cards for the connection, everything the server sends
 * that isn't a conversation, and the server's capabilities and settings beside it.
 */
const chat = useChatStore()
const { time, full } = useTimeFormat()
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const log = useQuery(() => serverLogQuery(chat.serverView))
const connection = useQuery(() => connectionQuery(chat.network?.id ?? ''))
const filter = ref('')
const mobileView = ref<'overview' | 'capabilities'>('overview')
useViewHistory('server-section', mobileView)
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
/** Routine runs fold into one row; a filter shows every matching line on its own. */
const items = computed<ServerLogItem[]>(() =>
  filter.value.trim()
    ? lines.value.map((line) => ({ type: 'line' as const, line }))
    : groupServerLog(lines.value),
)
/** “14:38–14:39”, or one time when the whole run shares it. */
function span(run: ServerLine[]) {
  const [from, to] = [time(run[0]!.time), time(run.at(-1)!.time)]
  return from === to ? from : `${from}–${to}`
}
const opened = ref(new Set<string>())
function toggleGroup(id: string) {
  const next = new Set(opened.value)
  if (!next.delete(id)) next.add(id)
  opened.value = next
}
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
  <div
    class="server-window relative flex-1 min-h-0 flex flex-col"
    :data-view="mobileView"
  >
    <AppSegmented
      v-model="mobileView"
      class="server-views mx-3 mt-2 max-w-[calc(100%_-_24px)] shrink-0 self-start overflow-x-auto [@media(pointer:coarse)]:[&_button]:min-h-[44px]"
      :label="t('collectionsUi.server.view')"
      :options="[
        { value: 'overview', label: t('collectionsUi.server.overview') },
        { value: 'capabilities', label: t('collectionsUi.server.capabilities') },
      ]"
    />
    <div class="server-content flex min-h-0 flex-1 flex-col">
      <ServerOverview
        v-if="chat.network"
        :key="chat.network.id"
        :network="chat.network"
        :info="connection.data.value"
      />
      <!-- The log scrolls on the left; capabilities and ISUPPORT sit in a column on the right. -->
      <div
        class="server-details mt-2 grid min-h-0 flex-1 border-t [border-top-style:solid] border-border/40 [grid-template-columns:minmax(0,1fr)_calc(320px*var(--interface-scale,1))]"
      >
        <section
          class="server-log-panel relative flex min-h-0 min-w-0 flex-col"
          :aria-label="t('collectionsUi.server.log')"
        >
          <div class="server-toolbar flex items-center gap-3 px-4 py-2.5 max-[700px]:px-3">
            <SearchField
              v-model="filter"
              class="server-filter w-[min(340px,_60%)]"
              :placeholder="t('collectionsUi.server.filterLines')"
              :aria-label="t('collectionsUi.server.filterLines')"
            />
            <span
              class="server-count text-[length:var(--ui-caption)] text-muted-foreground tabular-nums"
              >{{
                t('collectionsUi.server.lineCount', { count: n(lines.length) }, lines.length)
              }}</span
            >
          </div>
          <div
            ref="scroller"
            class="server-log flex-1 min-h-0 overflow-y-auto pb-3 font-code text-[length:var(--ui-small)] leading-[1.7] [touch-action:pan-y_pinch-zoom]"
            tabindex="0"
            role="log"
            :aria-label="
              t('collectionsUi.server.messagesFor', { server: chat.network?.name ?? '' })
            "
            @scroll.passive="onScroll"
          >
            <p
              v-if="log.isLoading.value && !lines.length"
              class="server-empty py-6 px-4 text-faint font-sans"
            >
              {{ t('collectionsUi.server.loading') }}
            </p>
            <p
              v-else-if="log.error.value"
              class="server-empty py-6 px-4 text-faint font-sans"
            >
              {{ errorMessage(log.error.value) }}
              <UiButton
                variant="link"
                size="text"
                @click="log.refetch()"
              >
                {{ t('common.tryAgain') }}
              </UiButton>
            </p>
            <div
              v-else-if="!lines.length"
              class="server-empty flex flex-col items-center gap-3 px-4 py-12 text-center font-sans text-[length:var(--ui-small)] text-muted-foreground"
            >
              <p class="m-0">
                {{
                  filter
                    ? t('collectionsUi.server.noLinesMatch', { query: filter })
                    : t('collectionsUi.server.emptyLog')
                }}
              </p>
              <UiButton
                v-if="filter"
                variant="secondary"
                size="sm"
                @click="filter = ''"
                >{{ t('collectionsUi.server.clearFilter') }}</UiButton
              >
            </div>
            <template
              v-for="item in items"
              :key="item.type === 'group' ? item.id : item.line.id"
            >
              <button
                v-if="item.type === 'group'"
                type="button"
                class="server-group grid w-full [grid-template-columns:6ch_12ch_minmax(0,_1fr)] gap-x-4 py-0.5 px-4 text-left text-muted-foreground hover:bg-accent/50 max-[700px]:[grid-template-columns:7ch_minmax(0,_1fr)]"
                :aria-expanded="opened.has(item.id)"
                @click="toggleGroup(item.id)"
              >
                <time
                  class="text-faint tabular-nums max-[700px]:hidden"
                  :datetime="item.lines[0]!.time"
                  :title="full(item.lines[0]!.time)"
                  >{{ time(item.lines[0]!.time) }}</time
                ><span class="text-right text-faint">×{{ n(item.lines.length) }}</span
                ><span class="flex min-w-0 items-center gap-1.5 font-sans"
                  ><AppIcon
                    :name="opened.has(item.id) ? 'down' : 'right'"
                    :size="13"
                    class="shrink-0"
                  /><span class="truncate"
                    >{{
                      t(
                        `collectionsUi.server.groups.${item.family}`,
                        { count: n(item.lines.length) },
                        item.lines.length,
                      )
                    }}
                    · {{ span(item.lines) }}</span
                  ></span
                >
              </button>
              <template v-if="item.type === 'line' || opened.has(item.id)">
                <div
                  v-for="line in item.type === 'line' ? [item.line] : item.lines"
                  :key="line.id"
                  class="server-line grid [grid-template-columns:6ch_12ch_minmax(0,_1fr)] gap-x-4 py-0.5 px-4 text-body hover:bg-accent/50 [&.is-error]:text-destructive [&.is-error_.server-command]:text-destructive [&.is-notice_.server-command]:text-warning [&.is-motd_.server-text]:text-muted-foreground [&.is-outgoing_.server-command]:text-primary [&.is-outgoing_.server-text]:text-primary max-[700px]:[grid-template-columns:7ch_minmax(0,_1fr)]"
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
              </template>
            </template>
          </div>
          <button
            v-if="!atBottom"
            class="jump-latest absolute bottom-3 left-1/2 [transform:translateX(-50%)] flex items-center gap-2 bg-primary text-primary-foreground py-1.5 px-3 rounded-[999px] text-[12px] font-[600] shadow-control whitespace-nowrap"
            @click="scrollEnd"
          >
            {{ t('collectionsUi.server.jumpToPresent')
            }}<AppIcon
              name="down"
              :size="14"
            />
          </button>
        </section>
        <ServerCapabilities
          v-if="chat.network"
          :key="chat.network.id"
          :network="chat.network"
          :info="connection.data.value"
          :loading="connection.isLoading.value"
        />
      </div>
    </div>
  </div>
</template>
<style scoped>
.server-window {
  container: server / inline-size;
}
.server-views {
  display: none;
}
/* Use the room left after navigation, including tablet landscape and resized panels. */
@container server (max-width: 1050px) {
  .server-views {
    display: inline-flex;
  }
  .server-content {
    overflow-y: auto;
  }
  .server-overview {
    display: grid;
    min-height: 0;
    flex: 0 0 auto;
  }
  .server-details {
    grid-template-columns: minmax(0, 1fr);
    min-height: min(240px, 40dvh);
  }
  [data-view='capabilities'] .server-details {
    min-height: 0;
  }
  .server-capabilities {
    border-left: 0;
  }
  [data-view='capabilities'] .server-overview,
  [data-view='overview'] .server-capabilities,
  [data-view='capabilities'] .server-log-panel {
    display: none;
  }
}
</style>
