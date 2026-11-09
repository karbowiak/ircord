<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { ConnectionInfo, Network } from '../../shared/types/chat'
import { latencyQuery } from '../queries/chat'

/**
 * The server page's summary cards: connection state; latency over the last half hour from this
 * device through IRCord to the IRC server, hop by hop and in total; a security grade for the
 * TLS connection; and the session (nick, capabilities, settings).
 */
const props = defineProps<{ network: Network; info?: ConnectionInfo }>()
const { full } = useTimeFormat()

// Latency isn't pushed; refresh it as often as the server is pinged while this page shows.
const latency = useQuery(() => latencyQuery(props.network.id))
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  timer = setInterval(() => {
    now.value = Date.now()
    if (!document.hidden) void latency.refetch()
  }, 20_000)
})
onBeforeUnmount(() => clearInterval(timer))

// Two hops: this device to IRCord (sampled in the browser) and IRCord to the IRC server
// (keep-alive pings in the backend). Their sum, matched by time, is the whole round trip.
const chat = useChatStore()
const link = useClientLink()
const ircSamples = computed(() => latency.data.value?.samples ?? [])
const deviceSamples = computed(() => link.samples.value)
const totalSamples = computed(() =>
  ircSamples.value.flatMap((s) => {
    const at = new Date(s.at).getTime()
    const near = deviceSamples.value.find(
      (d) => Math.abs(new Date(d.at).getTime() - at) <= link.intervalSeconds * 1000,
    )
    return near ? [{ at: s.at, ms: Math.round((s.ms + near.ms) * 10) / 10 }] : []
  }),
)
/** The latest sample, unless it's stale: a stalled hop shouldn't show its last good number. */
const fresh = (list: readonly { at: string; ms: number }[]) => {
  const last = list.at(-1)
  return last && now.value - new Date(last.at).getTime() <= 75_000 ? last.ms : undefined
}
const ircNow = computed(() => fresh(ircSamples.value))
const deviceNow = computed(() => fresh(deviceSamples.value))
const totalNow = computed(() =>
  ircNow.value !== undefined && deviceNow.value !== undefined
    ? ircNow.value + deviceNow.value
    : undefined,
)
const stats = (list: readonly { ms: number }[]) =>
  list.length
    ? {
        average: list.reduce((t, s) => t + s.ms, 0) / list.length,
        peak: Math.max(...list.map((s) => s.ms)),
      }
    : null
const totalStats = computed(() => stats(totalSamples.value))
const ms = (value?: number) => (value === undefined ? '—' : `${Math.round(value)} ms`)
const hops = {
  device: { label: 'You ↔ IRCord', dash: '5 3' },
  irc: { label: 'IRCord ↔ IRC', dash: '1.5 3' },
}
const chartSeries = computed(() => [
  {
    id: 'total',
    label: 'Total',
    samples: totalSamples.value,
    intervalSeconds: latency.data.value?.intervalSeconds ?? 20,
  },
  {
    id: 'device',
    ...hops.device,
    samples: deviceSamples.value,
    intervalSeconds: link.intervalSeconds,
  },
  {
    id: 'irc',
    ...hops.irc,
    samples: ircSamples.value,
    intervalSeconds: latency.data.value?.intervalSeconds ?? 20,
  },
])
const hasChart = computed(() => ircSamples.value.length > 1 || deviceSamples.value.length > 1)

// The live connection to IRCord, for the device's details.
const live = computed(
  () =>
    ({
      connected: 'Live',
      connecting: 'Connecting',
      reconnecting: 'Reconnecting',
      offline: 'Offline',
    })[chat.status],
)
const protocolLabel = computed(
  () =>
    ({ h2: 'HTTP/2', h3: 'HTTP/3', 'http/1.1': 'HTTP/1.1' })[link.protocol.value] ??
    (link.protocol.value || 'Unknown'),
)
const secure = location.protocol === 'https:'
const clockLabel = computed(() => {
  const offset = link.clockOffset.value
  if (offset === null) return 'Not measured yet'
  if (Math.abs(offset) < 1000) return 'In sync with the server'
  const seconds = (Math.abs(offset) / 1000).toFixed(1)
  return `${seconds} s ${offset > 0 ? 'behind' : 'ahead of'} the server`
})
const ago = (at: number | null) => {
  if (at === null) return 'none yet'
  const seconds = Math.max(0, Math.round((now.value - at) / 1000))
  return seconds < 5
    ? 'just now'
    : seconds < 90
      ? `${seconds} s ago`
      : `${Math.round(seconds / 60)} min ago`
}

const state = computed(
  () =>
    ({
      connected: 'Connected',
      connecting: 'Connecting',
      disconnected: 'Disconnected',
      error: 'Connection failed',
    })[props.network.status],
)
function since(iso?: string | null) {
  if (!iso) return ''
  const minutes = Math.max(0, Math.floor((now.value - new Date(iso).getTime()) / 60_000))
  if (minutes < 1) return 'just now'
  const days = Math.floor(minutes / 1440)
  const hours = Math.floor((minutes % 1440) / 60)
  const mins = minutes % 60
  return `for ${days ? `${days}d ${hours}h` : hours ? `${hours}h ${mins}m` : `${mins}m`}`
}

const tls = computed(() => props.info?.tls)
const grade = computed(() => (tls.value ? tls.value.grade : props.network.tls ? '' : 'F'))
const gradeTone = computed(() =>
  grade.value.startsWith('A')
    ? 'bg-success/15 text-success'
    : grade.value.startsWith('B')
      ? 'bg-warning/15 text-warning'
      : 'bg-destructive/15 text-destructive',
)
const expires = computed(() => {
  const end = tls.value?.notAfter
  if (!end) return ''
  const days = Math.round((new Date(end).getTime() - now.value) / 86_400_000)
  return days < 0 ? 'expired' : `in ${days} ${days === 1 ? 'day' : 'days'}`
})
const keyLabel = computed(() =>
  tls.value?.keyAlgorithm
    ? `${tls.value.keyAlgorithm}${tls.value.keyBits ? ` ${tls.value.keyBits}-bit` : ''}`
    : '',
)
const card =
  'server-card flex min-w-0 flex-col gap-1.5 rounded-lg border border-solid border-border/60 bg-card px-3.5 py-3'
const cardLabel =
  'm-0 flex items-center gap-1.5 text-[length:var(--ui-caption)] font-medium text-muted-foreground'
</script>
<template>
  <!-- The same two columns as the log and capabilities below (ServerWindow), with the same
       gutters, so the cards' edges line up with the content under them. -->
  <section
    class="server-overview grid shrink-0 pt-3 pb-1 [grid-template-columns:minmax(0,1fr)_calc(320px*var(--interface-scale,1))]"
    aria-label="Server connection"
  >
    <div
      class="overview-cards grid min-w-0 gap-3 px-4 [grid-template-columns:repeat(auto-fit,minmax(min(100%,calc(220px*var(--interface-scale,1))),1fr))] min-[1400px]:[grid-template-columns:minmax(0,0.8fr)_minmax(0,3fr)_minmax(0,1fr)] max-[700px]:px-3"
    >
      <div :class="card">
        <p :class="cardLabel">
          <AppIcon
            name="terminal"
            :size="13"
          />Status
        </p>
        <p class="m-0 flex items-center gap-2 text-[length:var(--ui-heading)] font-semibold">
          <i
            class="size-2 shrink-0 rounded-full"
            :class="{
              'bg-success': network.status === 'connected',
              'bg-warning': network.status === 'connecting',
              'bg-destructive': network.status === 'error',
              'bg-muted-foreground': network.status === 'disconnected',
            }"
            aria-hidden="true"
          />{{ state }}
        </p>
        <p
          class="m-0 truncate text-[length:var(--ui-small)] text-muted-foreground"
          :title="info?.connectedAt ? `Since ${full(info.connectedAt)}` : undefined"
        >
          <template v-if="network.status === 'connected' && info?.connectedAt"
            >{{ since(info.connectedAt) }} · </template
          ><span class="font-code">{{ info?.serverName || network.host }}</span>
        </p>
        <p
          v-if="info?.lastError && network.status !== 'connected'"
          class="m-0 line-clamp-2 text-[length:var(--ui-caption)] text-destructive"
        >
          {{ info.lastError }}
        </p>
      </div>

      <!-- Latency end to end: the total and each hop, over a chart of all three as the card's
         background. The content lets the pointer through to the chart, except its buttons. -->
      <div
        :class="[
          card,
          'server-latency relative overflow-hidden min-[900px]:col-span-2 min-[1400px]:col-span-1',
        ]"
      >
        <LatencyChart
          v-if="hasChart"
          underlay
          role="img"
          :aria-label="`Round trip over the last 30 minutes. Total: average ${ms(totalStats?.average)}, peak ${ms(totalStats?.peak)}. ${hops.device.label}: ${ms(deviceNow)} now. ${hops.irc.label}: ${ms(ircNow)} now.`"
          :series="chartSeries"
          :window-seconds="latency.data.value?.windowSeconds ?? 1800"
          :now="now"
        />
        <div
          class="pointer-events-none relative flex flex-1 flex-col gap-1.5 [&_button]:pointer-events-auto"
        >
          <!-- Header: the label, the path hop by hop, and the window. Each connector's line style
             matches its series in the chart. -->
          <div class="server-latency-heading flex min-w-0 items-center gap-3">
            <p :class="[cardLabel, 'shrink-0']">
              <AppIcon
                name="clock"
                :size="13"
              />Latency
            </p>
            <div
              class="server-hops flex min-w-0 flex-1 items-center gap-1.5 text-[length:var(--ui-caption)] [&_.hop-chip]:bg-card/85"
              aria-label="Connection path"
            >
              <UiPopover>
                <UiPopoverTrigger as-child>
                  <button
                    type="button"
                    class="hop-chip flex shrink-0 items-center gap-1.5 rounded-sm px-1.5 py-0.5 font-medium text-foreground hover:bg-accent! data-[state=open]:bg-accent! [@media(pointer:coarse)]:min-h-[44px]"
                    aria-label="Your device: show details about your connection to IRCord"
                  >
                    <i
                      class="size-1.5 rounded-full"
                      :class="
                        chat.status === 'connected'
                          ? 'bg-success'
                          : chat.status === 'offline'
                            ? 'bg-destructive'
                            : 'bg-warning'
                      "
                      aria-hidden="true"
                    />You<AppIcon
                      name="down"
                      :size="11"
                      class="text-muted-foreground"
                    />
                  </button>
                </UiPopoverTrigger>
                <UiPopoverContent
                  align="start"
                  class="w-[min(calc(340px*var(--interface-scale,1)),calc(100vw-16px))] p-3"
                  aria-label="Your connection to IRCord"
                >
                  <dl
                    class="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[length:var(--ui-caption)] [&_dd]:m-0 [&_dd]:text-body [&_dt]:text-muted-foreground"
                  >
                    <dt>Live updates</dt>
                    <dd>
                      {{ live
                      }}{{
                        chat.status === 'connected' && chat.liveSince
                          ? ` · ${since(new Date(chat.liveSince).toISOString())}`
                          : ''
                      }}
                    </dd>
                    <dt>Reconnects</dt>
                    <dd>{{ chat.reconnects }} since this tab opened</dd>
                    <dt>Last update</dt>
                    <dd>{{ ago(chat.lastEventAt) }}</dd>
                    <dt>Round trip</dt>
                    <dd>
                      {{ ms(deviceNow)
                      }}{{
                        stats(deviceSamples)
                          ? ` · avg ${ms(stats(deviceSamples)!.average)} · peak ${ms(stats(deviceSamples)!.peak)}`
                          : ''
                      }}
                    </dd>
                    <dt>Protocol</dt>
                    <dd>
                      {{ protocolLabel }} ·
                      {{ secure ? 'Encrypted (HTTPS)' : 'Unencrypted (HTTP)' }}
                    </dd>
                    <dt>Clock</dt>
                    <dd>{{ clockLabel }}</dd>
                  </dl>
                  <p
                    v-if="link.failing.value"
                    class="m-0 mt-2.5 text-[length:var(--ui-caption)] text-destructive"
                  >
                    IRCord isn't answering right now.
                  </p>
                </UiPopoverContent>
              </UiPopover>
              <span
                class="flex min-w-max flex-1 items-center gap-1"
                :aria-label="`${hops.device.label}: ${ms(deviceNow)}`"
                ><span
                  class="flex-1 border-t-2 [border-top-style:dashed] border-muted-foreground"
                  aria-hidden="true" /><span
                  class="hop-chip rounded-sm px-1 font-semibold whitespace-nowrap text-foreground tabular-nums"
                  >{{ ms(deviceNow) }}</span
                ><span
                  class="flex-1 border-t-2 [border-top-style:dashed] border-muted-foreground"
                  aria-hidden="true"
              /></span>
              <span
                class="hop-chip flex shrink-0 items-center gap-1.5 rounded-sm px-1.5 py-0.5 font-medium text-foreground"
                ><AppIcon
                  name="brand"
                  :size="13"
                />IRCord</span
              >
              <span
                class="flex min-w-max flex-1 items-center gap-1"
                :aria-label="`${hops.irc.label}: ${ms(ircNow)}`"
                ><span
                  class="flex-1 border-t-2 [border-top-style:dotted] border-muted-foreground"
                  aria-hidden="true" /><span
                  class="hop-chip rounded-sm px-1 font-semibold whitespace-nowrap text-foreground tabular-nums"
                  >{{ ms(ircNow) }}</span
                ><span
                  class="flex-1 border-t-2 [border-top-style:dotted] border-muted-foreground"
                  aria-hidden="true"
              /></span>
              <span
                class="hop-chip flex min-w-0 shrink items-center gap-1.5 rounded-sm px-1.5 py-0.5 font-medium text-foreground"
                ><AppIcon
                  name="terminal"
                  :size="13"
                  class="shrink-0"
                /><span class="truncate font-code">{{
                  info?.serverName || network.host
                }}</span></span
              >
            </div>
            <span
              class="server-latency-window shrink-0 text-[length:var(--ui-caption)] whitespace-nowrap text-faint"
              title="The last 30 minutes"
              >30 min</span
            >
          </div>
          <p
            class="-ml-1 m-0 flex w-fit items-baseline gap-2 rounded-sm bg-card/85 px-1 tabular-nums"
          >
            <span class="text-[length:var(--ui-heading)] leading-none font-bold tracking-tight">{{
              ms(totalNow)
            }}</span
            ><span class="text-[length:var(--ui-caption)] text-muted-foreground">total</span>
          </p>
          <p
            v-if="totalStats"
            class="-ml-1 m-0 mt-auto w-fit rounded-sm bg-card/85 px-1 text-[length:var(--ui-caption)] text-muted-foreground tabular-nums"
          >
            avg {{ ms(totalStats.average) }} · peak {{ ms(totalStats.peak) }}
          </p>
        </div>
      </div>

      <UiPopover>
        <UiPopoverTrigger as-child>
          <button
            type="button"
            :class="[
              card,
              'text-left transition-colors hover:border-border data-[state=open]:border-primary',
            ]"
            :aria-label="`Security grade ${grade || 'unknown'}. Show connection security details`"
          >
            <span :class="cardLabel">
              <AppIcon
                :name="tls || !network.tls ? 'lock' : 'shield'"
                :size="13" />Security
              <AppIcon
                name="down"
                :size="12"
                class="ml-auto"
            /></span>
            <span class="flex items-center gap-3">
              <span
                v-if="grade"
                class="grid h-10 min-w-10 place-items-center rounded-md px-1.5 text-[length:calc(20px*var(--interface-scale,1))] font-bold tracking-tight"
                :class="gradeTone"
                >{{ grade }}</span
              >
              <span class="flex min-w-0 flex-col">
                <span
                  class="truncate text-[length:var(--ui-small)] font-semibold text-foreground"
                  >{{
                    tls
                      ? `${tls.version} · ${tls.verified ? 'Verified' : 'Not verified'}`
                      : network.tls
                        ? 'Encrypted'
                        : 'Unencrypted'
                  }}</span
                >
                <span class="truncate text-[length:var(--ui-caption)] text-muted-foreground">{{
                  tls
                    ? tls.issuer || tls.cipher
                    : network.tls
                      ? 'Details arrive once connected'
                      : 'Traffic is readable on the network'
                }}</span>
              </span>
            </span>
          </button>
        </UiPopoverTrigger>
        <UiPopoverContent
          align="start"
          class="w-[min(calc(380px*var(--interface-scale,1)),calc(100vw-16px))] p-3"
          aria-label="Connection security"
        >
          <template v-if="tls">
            <ul
              v-if="tls.gradeNotes.length"
              class="m-0 mb-3 grid list-none gap-1.5 p-0 text-[length:var(--ui-small)] text-body"
            >
              <li
                v-for="note in tls.gradeNotes"
                :key="note"
                class="flex gap-2"
              >
                <AppIcon
                  name="alert"
                  :size="14"
                  class="mt-0.5 shrink-0 text-warning"
                />{{ note }}
              </li>
            </ul>
            <p
              v-else
              class="m-0 mb-3 flex gap-2 text-[length:var(--ui-small)] text-body"
            >
              <AppIcon
                name="check"
                :size="14"
                class="mt-0.5 shrink-0 text-success"
              />Modern protocol, trusted certificate and a strong key.
            </p>
            <dl
              class="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[length:var(--ui-caption)] [&_dd]:m-0 [&_dd]:wrap-anywhere [&_dd]:text-body [&_dt]:text-muted-foreground"
            >
              <dt>Protocol</dt>
              <dd>{{ tls.version }}</dd>
              <dt>Cipher</dt>
              <dd class="font-code">{{ tls.cipher }}</dd>
              <dt>Forward secrecy</dt>
              <dd>{{ tls.forwardSecrecy ? 'Yes' : 'No' }}</dd>
              <template v-if="keyLabel">
                <dt>Key</dt>
                <dd>{{ keyLabel }}</dd>
              </template>
              <template v-if="tls.signatureAlgorithm">
                <dt>Signature</dt>
                <dd>{{ tls.signatureAlgorithm }}</dd>
              </template>
              <template v-if="tls.subject">
                <dt>Issued to</dt>
                <dd>{{ tls.subject }}</dd>
              </template>
              <template v-if="tls.issuer">
                <dt>Issued by</dt>
                <dd>{{ tls.issuer }}</dd>
              </template>
              <template v-if="tls.notAfter">
                <dt>Valid until</dt>
                <dd>{{ full(tls.notAfter) }} ({{ expires }})</dd>
              </template>
              <template v-if="tls.certificateSha256">
                <dt>SHA-256</dt>
                <dd class="font-code text-[length:var(--ui-label)]">
                  {{ tls.certificateSha256 }}
                </dd>
              </template>
            </dl>
          </template>
          <p
            v-else
            class="m-0 text-[length:var(--ui-small)] text-body"
          >
            {{
              network.tls
                ? 'Connection security details arrive once IRCord connects.'
                : 'This network is set up without TLS, so messages and passwords travel unencrypted. Turn on TLS in the network settings if the server supports it.'
            }}
          </p>
        </UiPopoverContent>
      </UiPopover>
    </div>

    <!-- A transparent border matches the capabilities column's divider, so edges stay exact. -->
    <div
      class="server-session flex min-w-0 border-l [border-left-style:solid] border-transparent px-3 max-[1099px]:border-l-0 max-[1099px]:px-4 max-[1099px]:pt-3 max-[700px]:px-3"
    >
      <div :class="[card, 'flex-1']">
        <p :class="cardLabel">
          <AppIcon
            name="user"
            :size="13"
          />Session
        </p>
        <p class="m-0 truncate text-[length:var(--ui-heading)] font-semibold">
          {{ network.nick }}
        </p>
        <p class="m-0 wrap-anywhere text-[length:var(--ui-small)] text-muted-foreground">
          <span class="font-code">{{ network.host }}:{{ network.port }}</span>
          <template v-if="info">
            · {{ info.negotiatedCapabilities.length }} capabilities ·
            {{ Object.keys(info.isupport).length }} settings</template
          >
        </p>
      </div>
    </div>
  </section>
</template>
<style scoped>
.server-latency {
  container-type: inline-size;
}
@container server (max-width: 1050px) {
  .server-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
    align-content: start;
    padding: 12px 16px 16px;
  }
  .overview-cards {
    display: contents;
  }
  .server-latency {
    grid-column: auto;
  }
  .server-session {
    border-left: 0;
    padding: 0;
  }
}
@container server (max-width: 600px) {
  .server-overview {
    grid-template-columns: minmax(0, 1fr);
    padding: 12px;
  }
}
@container (max-width: 500px) {
  .server-latency-heading {
    flex-wrap: wrap;
    row-gap: 8px;
  }
  .server-hops {
    order: 3;
    flex-basis: 100%;
    flex-wrap: wrap;
  }
  .server-latency-window {
    margin-left: auto;
  }
}
</style>
