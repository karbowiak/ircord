<!-- Not Vapor: Reka's `as-child` trigger merges its props into a VNode slot. -->
<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { ConnectionInfo, Network } from '../../../shared/types/chat'
import { latencyQuery } from '../../queries/chat'

/**
 * The server page's summary cards: connection state; latency over the last half hour from this
 * device through IRCord to the IRC server, hop by hop and in total; a security grade for the
 * TLS connection; and the session (nick, capabilities, settings).
 */
const props = defineProps<{ network: Network; info?: ConnectionInfo }>()
const { full } = useTimeFormat()
const { t, n } = useI18n()

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
const ms = (value?: number) =>
  value === undefined
    ? t('collectionsUi.latency.noSample')
    : t('collectionsUi.latency.milliseconds', { value: n(Math.round(value)) })
const hops = computed(() => ({
  device: { label: t('collectionsUi.serverOverview.deviceToClient'), dash: '5 3' },
  irc: { label: t('collectionsUi.serverOverview.clientToIrc'), dash: '1.5 3' },
}))
const chartSeries = computed(() => [
  {
    id: 'total',
    label: t('collectionsUi.serverOverview.total'),
    samples: totalSamples.value,
    intervalSeconds: latency.data.value?.intervalSeconds ?? 20,
  },
  {
    id: 'device',
    ...hops.value.device,
    samples: deviceSamples.value,
    intervalSeconds: link.intervalSeconds,
  },
  {
    id: 'irc',
    ...hops.value.irc,
    samples: ircSamples.value,
    intervalSeconds: latency.data.value?.intervalSeconds ?? 20,
  },
])
const hasChart = computed(() => ircSamples.value.length > 1 || deviceSamples.value.length > 1)

// The live connection to IRCord, for the device's details.
const live = computed(() => {
  const key = {
    connected: 'live.connected',
    connecting: 'live.connecting',
    syncing: 'live.syncing',
    reconnecting: 'live.reconnecting',
    offline: 'live.offline',
  }[chat.status]
  return t('collectionsUi.serverOverview.' + key)
})
const protocolLabel = computed(() => {
  return (
    {
      h2: 'HTTP/2',
      h3: 'HTTP/3',
      'http/1.1': 'HTTP/1.1',
    }[link.protocol.value as 'h2' | 'h3' | 'http/1.1'] ??
    link.protocol.value ??
    t('collectionsUi.serverOverview.unknown')
  )
})
const secure = location.protocol === 'https:'
const clockLabel = computed(() => {
  const offset = link.clockOffset.value
  if (offset === null) return t('collectionsUi.serverOverview.clockNotMeasured')
  if (Math.abs(offset) < 1000) return t('collectionsUi.serverOverview.clockInSync')
  const seconds = n(Math.abs(offset) / 1000, { maximumFractionDigits: 1 })
  return t(
    offset > 0
      ? 'collectionsUi.serverOverview.clockBehind'
      : 'collectionsUi.serverOverview.clockAhead',
    { seconds },
  )
})
const ago = (at: number | null) => {
  if (at === null) return t('collectionsUi.serverOverview.noneYet')
  const seconds = Math.max(0, Math.round((now.value - at) / 1000))
  if (seconds < 5) return t('collectionsUi.serverOverview.justNow')
  if (seconds < 90)
    return t('collectionsUi.serverOverview.secondsAgo', { count: n(seconds) }, seconds)
  const minutes = Math.round(seconds / 60)
  return t('collectionsUi.serverOverview.minutesAgo', { count: n(minutes) }, minutes)
}

const state = computed(() => {
  const key = {
    connected: 'connection.connected',
    connecting: 'connection.connecting',
    disconnected: 'connection.disconnected',
    error: 'connection.failed',
  }[props.network.status]
  return t('collectionsUi.serverOverview.' + key)
})
function since(iso?: string | null) {
  if (!iso) return ''
  const minutes = Math.max(0, Math.floor((now.value - new Date(iso).getTime()) / 60_000))
  if (minutes < 1) return t('collectionsUi.serverOverview.justNow')
  const days = Math.floor(minutes / 1440)
  const hours = Math.floor((minutes % 1440) / 60)
  const mins = minutes % 60
  const duration = [
    days ? t('collectionsUi.serverOverview.days', { count: n(days) }, days) : '',
    hours ? t('collectionsUi.serverOverview.hours', { count: n(hours) }, hours) : '',
    mins ? t('collectionsUi.serverOverview.minutes', { count: n(mins) }, mins) : '',
  ]
    .filter(Boolean)
    .join(' ')
  return t('collectionsUi.serverOverview.forDuration', { duration })
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
  return days < 0
    ? t('collectionsUi.serverOverview.expired')
    : t('collectionsUi.serverOverview.expiresInDays', { count: n(days) }, days)
})
const keyLabel = computed(() =>
  tls.value?.keyAlgorithm
    ? `${tls.value.keyAlgorithm}${tls.value.keyBits ? ` ${n(tls.value.keyBits)}-bit` : ''}`
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
    :aria-label="t('collectionsUi.serverOverview.serverConnection')"
  >
    <div
      class="overview-cards grid min-w-0 gap-3 px-4 [grid-template-columns:repeat(auto-fit,minmax(min(100%,calc(220px*var(--interface-scale,1))),1fr))] min-[1400px]:[grid-template-columns:minmax(0,0.8fr)_minmax(0,3fr)_minmax(0,1fr)] max-[700px]:px-3"
    >
      <div :class="card">
        <p :class="cardLabel">
          <AppIcon
            name="terminal"
            :size="13"
          />{{ t('collectionsUi.serverOverview.status') }}
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
          :title="
            info?.connectedAt
              ? t('collectionsUi.serverOverview.sinceDate', { date: full(info.connectedAt) })
              : undefined
          "
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
          :aria-label="
            t('collectionsUi.serverOverview.roundTripSummary', {
              average: ms(totalStats?.average),
              peak: ms(totalStats?.peak),
              deviceLabel: hops.device.label,
              device: ms(deviceNow),
              serverLabel: hops.irc.label,
              server: ms(ircNow),
            })
          "
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
              />{{ t('collectionsUi.serverOverview.latency') }}
            </p>
            <div
              class="server-hops flex min-w-0 flex-1 items-center gap-1.5 text-[length:var(--ui-caption)] [&_.hop-chip]:bg-card/85"
              :aria-label="t('collectionsUi.serverOverview.connectionPath')"
            >
              <UiPopover>
                <UiPopoverTrigger as-child>
                  <button
                    type="button"
                    class="hop-chip flex shrink-0 items-center gap-1.5 rounded-sm px-1.5 py-0.5 font-medium text-foreground hover:bg-accent! data-[state=open]:bg-accent! [@media(pointer:coarse)]:min-h-[44px]"
                    :aria-label="t('collectionsUi.serverOverview.deviceDetails')"
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
                    />{{ t('collectionsUi.serverOverview.you')
                    }}<AppIcon
                      name="down"
                      :size="11"
                      class="text-muted-foreground"
                    />
                  </button>
                </UiPopoverTrigger>
                <UiPopoverContent
                  align="start"
                  class="w-[min(calc(340px*var(--interface-scale,1)),calc(100vw-16px))] p-3"
                  :aria-label="t('collectionsUi.serverOverview.connectionToClient')"
                >
                  <dl
                    class="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[length:var(--ui-caption)] [&_dd]:m-0 [&_dd]:text-body [&_dt]:text-muted-foreground"
                  >
                    <dt>{{ t('collectionsUi.serverOverview.liveUpdates') }}</dt>
                    <dd>
                      {{
                        chat.status === 'connected' && chat.liveSince
                          ? t('collectionsUi.serverOverview.liveSince', {
                              state: live,
                              duration: since(new Date(chat.liveSince).toISOString()),
                            })
                          : live
                      }}
                    </dd>
                    <dt>{{ t('collectionsUi.serverOverview.reconnects') }}</dt>
                    <dd>
                      {{
                        t(
                          'collectionsUi.serverOverview.reconnectCount',
                          { count: n(chat.reconnects) },
                          chat.reconnects,
                        )
                      }}
                    </dd>
                    <dt>{{ t('collectionsUi.serverOverview.lastUpdate') }}</dt>
                    <dd>{{ ago(chat.lastEventAt) }}</dd>
                    <dt>{{ t('collectionsUi.serverOverview.roundTrip') }}</dt>
                    <dd>
                      {{
                        stats(deviceSamples)
                          ? t('collectionsUi.serverOverview.roundTripStats', {
                              current: ms(deviceNow),
                              average: ms(stats(deviceSamples)!.average),
                              peak: ms(stats(deviceSamples)!.peak),
                            })
                          : ms(deviceNow)
                      }}
                    </dd>
                    <dt>{{ t('collectionsUi.serverOverview.protocolLabel') }}</dt>
                    <dd>
                      {{ protocolLabel }} ·
                      {{
                        t(
                          secure
                            ? 'collectionsUi.serverOverview.encryptedProtocol'
                            : 'collectionsUi.serverOverview.unencryptedProtocol',
                          { protocol: secure ? 'HTTPS' : 'HTTP' },
                        )
                      }}
                    </dd>
                    <dt>{{ t('collectionsUi.serverOverview.clock') }}</dt>
                    <dd>{{ clockLabel }}</dd>
                  </dl>
                  <p
                    v-if="link.failing.value"
                    class="m-0 mt-2.5 text-[length:var(--ui-caption)] text-destructive"
                  >
                    {{ t('collectionsUi.serverOverview.clientNotAnswering') }}
                  </p>
                </UiPopoverContent>
              </UiPopover>
              <span
                class="flex min-w-max flex-1 items-center gap-1"
                :aria-label="
                  t('collectionsUi.serverOverview.hopValue', {
                    hop: hops.device.label,
                    value: ms(deviceNow),
                  })
                "
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
                :aria-label="
                  t('collectionsUi.serverOverview.hopValue', {
                    hop: hops.irc.label,
                    value: ms(ircNow),
                  })
                "
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
              :title="t('collectionsUi.serverOverview.lastThirtyMinutes')"
              >{{ t('collectionsUi.serverOverview.thirtyMinutes') }}</span
            >
          </div>
          <p
            class="-ml-1 m-0 flex w-fit items-baseline gap-2 rounded-sm bg-card/85 px-1 tabular-nums"
          >
            <span class="text-[length:var(--ui-heading)] leading-none font-bold tracking-tight">{{
              ms(totalNow)
            }}</span
            ><span class="text-[length:var(--ui-caption)] text-muted-foreground">{{
              t('collectionsUi.serverOverview.total')
            }}</span>
          </p>
          <p
            v-if="totalStats"
            class="-ml-1 m-0 mt-auto w-fit rounded-sm bg-card/85 px-1 text-[length:var(--ui-caption)] text-muted-foreground tabular-nums"
          >
            {{
              t('collectionsUi.serverOverview.averagePeak', {
                average: ms(totalStats.average),
                peak: ms(totalStats.peak),
              })
            }}
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
          >
            <span :class="cardLabel">
              <AppIcon
                :name="tls || !network.tls ? 'lock' : 'shield'"
                :size="13" />{{ t('collectionsUi.serverOverview.security') }}
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
                      ? t('collectionsUi.serverOverview.tlsSummary', {
                          version: tls.version,
                          verification: t(
                            tls.verified
                              ? 'collectionsUi.serverOverview.verified'
                              : 'collectionsUi.serverOverview.notVerified',
                          ),
                        })
                      : network.tls
                        ? t('collectionsUi.serverOverview.encrypted')
                        : t('collectionsUi.serverOverview.unencrypted')
                  }}</span
                >
                <span class="truncate text-[length:var(--ui-caption)] text-muted-foreground">{{
                  tls
                    ? tls.issuer || tls.cipher
                    : network.tls
                      ? t('collectionsUi.serverOverview.tlsDetailsWhenConnected')
                      : t('collectionsUi.serverOverview.noTlsSummary')
                }}</span>
              </span>
            </span>
          </button>
        </UiPopoverTrigger>
        <UiPopoverContent
          align="start"
          class="w-[min(calc(380px*var(--interface-scale,1)),calc(100vw-16px))] p-3"
          :aria-label="t('collectionsUi.serverOverview.connectionSecurity')"
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
              />{{ t('collectionsUi.serverOverview.modernTls') }}
            </p>
            <dl
              class="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[length:var(--ui-caption)] [&_dd]:m-0 [&_dd]:wrap-anywhere [&_dd]:text-body [&_dt]:text-muted-foreground"
            >
              <dt>{{ t('collectionsUi.serverOverview.protocolLabel') }}</dt>
              <dd>{{ tls.version }}</dd>
              <dt>{{ t('collectionsUi.serverOverview.cipher') }}</dt>
              <dd class="font-code">{{ tls.cipher }}</dd>
              <dt>{{ t('collectionsUi.serverOverview.forwardSecrecy') }}</dt>
              <dd>
                {{
                  t(
                    tls.forwardSecrecy
                      ? 'collectionsUi.serverOverview.yes'
                      : 'collectionsUi.serverOverview.no',
                  )
                }}
              </dd>
              <template v-if="keyLabel">
                <dt>{{ t('collectionsUi.serverOverview.key') }}</dt>
                <dd>{{ keyLabel }}</dd>
              </template>
              <template v-if="tls.signatureAlgorithm">
                <dt>{{ t('collectionsUi.serverOverview.signature') }}</dt>
                <dd>{{ tls.signatureAlgorithm }}</dd>
              </template>
              <template v-if="tls.subject">
                <dt>{{ t('collectionsUi.serverOverview.issuedTo') }}</dt>
                <dd>{{ tls.subject }}</dd>
              </template>
              <template v-if="tls.issuer">
                <dt>{{ t('collectionsUi.serverOverview.issuedBy') }}</dt>
                <dd>{{ tls.issuer }}</dd>
              </template>
              <template v-if="tls.notAfter">
                <dt>{{ t('collectionsUi.serverOverview.validUntil') }}</dt>
                <dd>
                  {{
                    t('collectionsUi.serverOverview.validUntilValue', {
                      date: full(tls.notAfter),
                      expires,
                    })
                  }}
                </dd>
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
                ? t('collectionsUi.serverOverview.securityDetailsWhenConnected')
                : t('collectionsUi.serverOverview.tlsDisabledHelp')
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
          />{{ t('collectionsUi.serverOverview.session') }}
        </p>
        <p class="m-0 truncate text-[length:var(--ui-heading)] font-semibold">
          {{ network.nick }}
        </p>
        <p class="m-0 wrap-anywhere text-[length:var(--ui-small)] text-muted-foreground">
          <span class="font-code">{{ network.host }}:{{ network.port }}</span>
          <template v-if="info">
            ·
            {{
              t(
                'collectionsUi.serverOverview.capabilitiesCount',
                { count: n(info.negotiatedCapabilities.length) },
                info.negotiatedCapabilities.length,
              )
            }}
            ·
            {{
              t(
                'collectionsUi.serverOverview.settingsCount',
                { count: n(Object.keys(info.isupport).length) },
                Object.keys(info.isupport).length,
              )
            }}</template
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
