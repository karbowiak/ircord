<script lang="ts">
export interface LatencySeries {
  id: string
  label: string
  samples: readonly { at: string; ms: number }[]
  intervalSeconds: number
  /** SVG dash pattern; the headline series is solid. */
  dash?: string
}
</script>
<script setup lang="ts" vapor>
/**
 * Round-trip times over the window (the last half hour) on a shared time axis and scale, one
 * line per series; missing samples show as gaps. The first series is the headline: an accent
 * line with an area under it. The others are thinner, told apart by dash pattern as well as
 * by name. Hovering shows every series' value at that moment. `underlay` fills its container
 * behind other content (a card's background), with the readout inside it.
 */
const props = defineProps<{
  series: LatencySeries[]
  windowSeconds: number
  now: number
  underlay?: boolean
}>()
const { time } = useTimeFormat()
const { t, n } = useI18n()
const width = 300
const height = 80
const pad = 4
const start = computed(() => props.now - props.windowSeconds * 1000)
const visible = (s: { at: string }) => new Date(s.at).getTime() >= start.value
const peak = computed(
  () =>
    Math.max(10, ...props.series.flatMap((line) => line.samples.filter(visible).map((s) => s.ms))) *
    1.12,
)
const x = (at: string) =>
  ((new Date(at).getTime() - start.value) / (props.windowSeconds * 1000)) * width
const y = (ms: number) => height - pad - (ms / peak.value) * (height - pad * 2)

type Sample = { at: string; ms: number }
/** Runs of consecutive samples; more than two missed intervals starts a new run. */
function runs(line: LatencySeries) {
  const out: Sample[][] = []
  let last = 0
  for (const s of line.samples) {
    const at = new Date(s.at).getTime()
    if (at < start.value) continue
    if (!out.length || at - last > line.intervalSeconds * 1000 * 2.5) out.push([])
    out.at(-1)!.push(s)
    last = at
  }
  return out
}
const path = (run: Sample[]) =>
  run.map((s, i) => `${i ? 'L' : 'M'}${x(s.at).toFixed(1)},${y(s.ms).toFixed(1)}`).join('')
const area = (run: Sample[]) =>
  `${path(run)}L${x(run.at(-1)!.at).toFixed(1)},${height}L${x(run[0]!.at).toFixed(1)},${height}Z`
const drawn = computed(() => props.series.map((line) => ({ line, runs: runs(line) })))

/** The hovered moment, and each series' nearest sample to it (within two intervals). */
const hoverAt = ref<number | null>(null)
const box = ref<HTMLElement | null>(null)
function move(event: PointerEvent) {
  const rect = box.value?.getBoundingClientRect()
  if (!rect) return
  const at = start.value + ((event.clientX - rect.left) / rect.width) * props.windowSeconds * 1000
  // Snap to the headline series' nearest sample, so the readout lines up with a real point.
  const head = props.series[0]?.samples.filter(visible) ?? []
  const snap = head.reduce<Sample | null>(
    (best, s) =>
      !best || Math.abs(new Date(s.at).getTime() - at) < Math.abs(new Date(best.at).getTime() - at)
        ? s
        : best,
    null,
  )
  hoverAt.value = snap ? new Date(snap.at).getTime() : at
}
const readout = computed(() => {
  const at = hoverAt.value
  if (at === null) return []
  return props.series.map((line) => {
    const near = line.samples.reduce<Sample | null>((best, s) => {
      const d = Math.abs(new Date(s.at).getTime() - at)
      return d <= line.intervalSeconds * 2000 &&
        (!best || d < Math.abs(new Date(best.at).getTime() - at))
        ? s
        : best
    }, null)
    return { line, sample: near }
  })
})
const hoverX = computed(() =>
  hoverAt.value === null ? 0 : ((hoverAt.value - start.value) / (props.windowSeconds * 1000)) * 100,
)
</script>
<template>
  <div
    ref="box"
    class="latency-chart w-full"
    :class="underlay ? 'absolute inset-x-0 bottom-0 h-[62%]' : 'relative h-20'"
    @pointermove="move"
    @pointerleave="hoverAt = null"
  >
    <svg
      :viewBox="`0 0 ${width} ${height}`"
      preserveAspectRatio="none"
      class="size-full overflow-visible"
      aria-hidden="true"
    >
      <line
        x1="0"
        :x2="width"
        :y1="height - 0.5"
        :y2="height - 0.5"
        class="stroke-border"
        stroke-width="1"
        vector-effect="non-scaling-stroke"
      />
      <!-- Headline last, so it draws over the hops. -->
      <g
        v-for="({ line, runs: lineRuns }, index) in [...drawn].reverse()"
        :key="line.id"
      >
        <template v-if="index === drawn.length - 1">
          <path
            v-for="(run, i) in lineRuns"
            :key="`a${i}`"
            :d="area(run)"
            :class="underlay ? 'fill-primary/20' : 'fill-primary/12'"
          />
        </template>
        <path
          v-for="(run, i) in lineRuns"
          :key="i"
          :d="path(run)"
          fill="none"
          :class="index === drawn.length - 1 ? 'stroke-primary' : 'stroke-muted-foreground'"
          :stroke-width="index === drawn.length - 1 ? 2 : 1.5"
          :stroke-dasharray="line.dash"
          stroke-linejoin="round"
          stroke-linecap="round"
          vector-effect="non-scaling-stroke"
        />
      </g>
      <line
        v-if="hoverAt !== null"
        :x1="(hoverX / 100) * width"
        :x2="(hoverX / 100) * width"
        y1="0"
        :y2="height"
        class="stroke-muted-foreground"
        stroke-width="1"
        stroke-dasharray="2 2"
        vector-effect="non-scaling-stroke"
      />
    </svg>
    <template v-if="hoverAt !== null">
      <span
        v-for="{ line, sample } in readout.filter((r) => r.sample)"
        :key="line.id"
        class="pointer-events-none absolute size-2 -translate-1/2 rounded-full ring-2 ring-card"
        :class="line.id === series[0]?.id ? 'bg-primary' : 'bg-muted-foreground'"
        :style="{ left: `${hoverX}%`, top: `${(y(sample!.ms) / height) * 100}%` }"
      />
      <div
        class="pointer-events-none absolute z-10 grid -translate-x-1/2 gap-0.5 rounded-sm bg-popover px-2 py-1.5 text-[length:var(--ui-caption)] whitespace-nowrap text-foreground shadow-popover tabular-nums"
        :class="underlay ? 'bottom-1' : 'bottom-full mb-1.5'"
        :style="{ left: `${Math.min(85, Math.max(15, hoverX))}%` }"
      >
        <span class="text-muted-foreground">{{ time(new Date(hoverAt).toISOString()) }}</span>
        <span
          v-for="{ line, sample } in readout"
          :key="line.id"
          class="flex items-center justify-between gap-3"
          ><span class="text-muted-foreground">{{ line.label }}</span
          ><strong class="font-semibold">{{
            sample
              ? t('collectionsUi.latency.milliseconds', { value: n(Math.round(sample.ms)) })
              : t('collectionsUi.latency.noSample')
          }}</strong></span
        >
      </div>
    </template>
  </div>
</template>
