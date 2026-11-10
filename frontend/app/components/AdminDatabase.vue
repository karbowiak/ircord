<script setup vapor lang="ts">
import { useQuery } from '@pinia/colada'
import { useIntervalFn } from '@vueuse/core'
import { administrationDatabaseQuery } from '../queries/administration'
import { errorText } from '../utils/api'
import AdminFacts from './AdminFacts.vue'

const { t, n } = useI18n()
const { full } = useTimeFormat()
const query = useQuery(administrationDatabaseQuery)
const data = computed(() => query.data.value)
const refused = useAdministrationRefusal()
watch(() => query.error.value, refused)
const live = ref(true)
const refreshing = ref(false)
const recentWaits = ref<number | null>(null)
const recentWaitMS = ref<number | null>(null)
watch(data, (current, previous) => {
  recentWaits.value = recentWaitMS.value = null
  if (!current || !previous || current.startedAt !== previous.startedAt) return
  const elapsed = Date.parse(current.sampledAt) - Date.parse(previous.sampledAt)
  if (elapsed <= 0 || current.pool.waitCount < previous.pool.waitCount) return
  recentWaits.value = ((current.pool.waitCount - previous.pool.waitCount) * 60_000) / elapsed
  recentWaitMS.value = Math.max(0, current.pool.waitTotalMs - previous.pool.waitTotalMs)
})
async function refresh() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    await query.refetch()
  } catch {
    // The query error is shown beside the retained sample.
  } finally {
    refreshing.value = false
  }
}
useIntervalFn(() => {
  if (live.value && document.visibilityState === 'visible') void refresh().catch(() => {})
}, 10_000)
const unknown = () => t('administration.db.notMeasured')
const bytes = (value: number | null | undefined) => {
  if (value == null) return unknown()
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${n(value, { maximumFractionDigits: unit ? 1 : 0 })} ${units[unit]}`
}
const ms = (value: number) =>
  t('administration.db.milliseconds', { value: n(value, { maximumFractionDigits: 1 }) })
const poolRows = computed(() => {
  const pool = data.value?.pool
  return pool
    ? [
        { label: t('administration.db.connectionsInUse'), value: n(pool.inUse) },
        { label: t('administration.db.connectionsIdle'), value: n(pool.idle) },
        { label: t('administration.db.connectionsOpen'), value: n(pool.open) },
        {
          label: t('administration.db.connectionLimit'),
          value: pool.maxOpen ? n(pool.maxOpen) : t('administration.db.unlimited'),
        },
        { label: t('administration.db.totalWaits'), value: n(pool.waitCount) },
        { label: t('administration.db.totalWaitTime'), value: ms(pool.waitTotalMs) },
        {
          label: t('administration.db.recentWaits'),
          value:
            recentWaits.value === null
              ? unknown()
              : n(recentWaits.value, { maximumFractionDigits: 1 }),
        },
        {
          label: t('administration.db.recentWaitTime'),
          value: recentWaitMS.value === null ? unknown() : ms(recentWaitMS.value),
        },
      ]
    : []
})
const storageRows = computed(() =>
  data.value?.available
    ? [
        { label: t('administration.db.allocated'), value: bytes(data.value.allocatedBytes) },
        { label: t('administration.db.reusable'), value: bytes(data.value.reusableBytes) },
        { label: t('administration.db.file'), value: bytes(data.value.fileBytes) },
        { label: t('administration.db.wal'), value: bytes(data.value.walBytes) },
      ]
    : [],
)
const configRows = computed(() =>
  data.value?.available
    ? [
        {
          label: t('administration.db.engine'),
          value: `SQLite ${data.value.version ?? ''}`.trim(),
        },
        {
          label: t('administration.db.journal'),
          value: data.value.journalMode?.toUpperCase() ?? unknown(),
        },
        {
          label: t('administration.db.durability'),
          value: data.value.synchronous?.toUpperCase() ?? unknown(),
        },
        {
          label: t('administration.db.foreignKeys'),
          value: t(
            data.value.foreignKeys ? 'administration.db.enabled' : 'administration.db.disabled',
          ),
        },
        { label: t('administration.db.busyTimeout'), value: ms(data.value.busyTimeoutMs) },
        {
          label: t('administration.db.autoCheckpoint'),
          value: t('administration.db.pages', { count: n(data.value.autoCheckpointPages) }),
        },
      ]
    : [],
)
</script>
<template>
  <div class="grid gap-7">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="m-0 max-w-[55ch] text-[13px] text-muted-foreground">
        {{ t('administration.db.intro') }}
      </p>
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="refreshing"
        @click="refresh"
      >
        <AppIcon
          name="refresh"
          :size="14"
        />{{ t('administration.db.refresh') }}
      </UiButton>
    </div>
    <div
      class="flex flex-wrap items-center justify-between gap-3 text-[13px] text-muted-foreground"
    >
      <span v-if="data">{{ t('administration.db.sampled', { time: full(data.sampledAt) }) }}</span>
      <label class="flex items-center gap-2">
        <AppSwitch
          v-model="live"
          :aria-label="t('administration.db.live')"
        />{{ t('administration.db.live') }}
      </label>
    </div>
    <UiAlert
      v-if="query.error.value"
      variant="destructive"
      presentation="inline"
    >
      {{ errorText(query.error.value, t) }}
    </UiAlert>
    <SettingsDescription v-else-if="!data">{{ t('common.loading') }}</SettingsDescription>
    <template v-if="data">
      <UiAlert
        v-if="!data.available"
        presentation="inline"
        >{{ t('administration.db.unavailable') }}</UiAlert
      >
      <SettingGroup :title="t('administration.db.pool')">
        <AdminFacts :rows="poolRows" />
        <p class="m-0 text-[13px] leading-relaxed text-muted-foreground">
          {{ t('administration.db.waitHelp') }}
        </p>
        <p class="m-0 text-[13px] text-faint">
          {{ t('administration.db.countersSince', { time: full(data.startedAt) }) }}
        </p>
      </SettingGroup>
      <SettingGroup
        v-if="data.available"
        :title="t('administration.db.storage')"
      >
        <AdminFacts :rows="storageRows" />
        <p class="m-0 text-[13px] leading-relaxed text-muted-foreground">
          {{ t('administration.db.storageHelp') }}
        </p>
      </SettingGroup>
      <SettingGroup
        v-if="data.available"
        :title="t('administration.db.configuration')"
      >
        <AdminFacts :rows="configRows" />
      </SettingGroup>
    </template>
  </div>
</template>
