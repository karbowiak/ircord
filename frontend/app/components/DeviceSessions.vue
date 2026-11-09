<script setup lang="ts">
/** Settings → Account → Devices & sessions (#85): where this account is signed in. */
import { useQuery, useQueryCache } from '@pinia/colada'
import { devicesQuery } from '../queries/devices'
import {
  deviceName,
  exactTime,
  mobileDevice,
  notificationText,
  presence,
  relativeTime,
  type Device,
} from '../../shared/utils/devices'

const auth = useAuthStore()
const notifications = useNotificationStore()
const { confirm } = usePrompt()
const { t } = useI18n()
const queryCache = useQueryCache()
const list = useQuery(devicesQuery)
const devices = computed(() => {
  const all = list.data.value?.devices ?? []
  return [...all.filter((d) => d.current), ...all.filter((d) => !d.current)]
})
// Relative times are rendered against this clock, ticked once a minute rather than per second.
const now = ref(Date.now())
const tick = setInterval(() => (now.value = Date.now()), 60_000)
onScopeDispose(() => clearInterval(tick))

// Per device, so finishing one sign-out never re-enables another that is still in flight.
const busy = ref(new Set<string>())
const setBusy = (id: string, on: boolean) => {
  const next = new Set(busy.value)
  if (on) next.add(id)
  else next.delete(id)
  busy.value = next
}
const errors = ref<Record<string, string>>({})
const notice = ref('')

async function signOut(device: Device) {
  const name = deviceName(device)
  const ok = await confirm(
    device.current
      ? {
          title: t('devices.signOutThis.title'),
          body: t('devices.signOutThis.body'),
          confirm: t('common.signOut'),
          danger: true,
        }
      : {
          title: t('devices.signOutOther.title', { name }),
          body: t('devices.signOutOther.body'),
          confirm: t('devices.signOutOther.confirm'),
          danger: true,
        },
  )
  if (!ok) return
  setBusy(device.id, true)
  errors.value = { ...errors.value, [device.id]: '' }
  notice.value = ''
  try {
    await apiClient().signOutDevice(device.id)
    if (device.current) return auth.signedOut()
    notice.value = t('devices.signedOut', { name })
  } catch (error) {
    if (errorStatus(error) === 404) {
      if (device.current) return auth.signedOut()
      notice.value = t('devices.alreadySignedOut', { name })
    } else {
      // The device is still signed in; say so rather than dropping it from the list.
      errors.value = {
        ...errors.value,
        [device.id]: t('devices.signOutFailed', { name, error: errorText(error) }),
      }
    }
  } finally {
    setBusy(device.id, false)
    void queryCache.invalidateQueries({ key: devicesQuery.key }).catch(() => {})
  }
}
</script>
<template>
  <section
    class="setting-group grid gap-3.5"
    aria-labelledby="devices-heading"
  >
    <h3 id="devices-heading">{{ $t('devices.heading') }}</h3>
    <FieldHint>{{ $t('devices.intro') }}</FieldHint>
    <FieldHint
      v-if="list.isPending.value"
      role="status"
    >
      {{ $t('devices.loading') }}
    </FieldHint>
    <div
      v-else-if="list.error.value && !list.data.value"
      class="setting-row flex items-center justify-between gap-4"
    >
      <UiAlert
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ $t('devices.loadFailed', { error: errorText(list.error.value) }) }}
      </UiAlert>
      <UiButton
        variant="secondary"
        size="sm"
        @click="list.refetch()"
      >
        {{ $t('common.tryAgain') }}
      </UiButton>
    </div>
    <FieldHint v-else-if="!devices.length">{{ $t('devices.none') }}</FieldHint>
    <ul
      v-else
      class="device-list flex flex-col gap-1 mt-1 mb-0 mx-0 p-0 list-none"
      :aria-label="$t('devices.listLabel')"
    >
      <li
        v-for="device in devices"
        :key="device.id"
        class="device-row flex items-center gap-3 py-2.5 px-3 rounded-md bg-field text-muted-foreground max-[640px]:flex-wrap max-[640px]:[&_[data-slot=button]]:ml-8"
        :aria-label="device.current ? $t('devices.currentRow', { name: deviceName(device) }) : deviceName(device)"
      >
        <AppIcon
          :name="mobileDevice(device) ? 'smartphone' : 'monitor'"
          :size="20"
        />
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong
            >{{ deviceName(device)
            }}<UiBadge
              v-if="device.current"
              variant="secondary"
              class="device-badge ml-2 border-0 align-[1px]"
              >{{ $t('devices.thisDevice') }}</UiBadge
            ></strong
          ><small>
            <time
              v-if="device.lastSeenAt && !device.current"
              :datetime="device.lastSeenAt"
              :title="exactTime(device.lastSeenAt)"
              >{{ presence(device, now) }}</time
            ><span v-else>{{ presence(device, now) }}</span>
            ·
            <time
              :datetime="device.signedInAt"
              :title="exactTime(device.signedInAt)"
              >{{ $t('devices.signedIn', { time: relativeTime(device.signedInAt, now) }) }}</time
            >
            · {{ notificationText(device, notifications.mode) }}</small
          >
          <details
            class="device-details text-[12px] text-faint [&amp;_summary]:w-fit [&amp;_summary]:cursor-pointer [&amp;_dl]:grid [&amp;_dl]:grid-cols-[auto_1fr] [&amp;_dl]:gap-y-0.5 [&amp;_dl]:gap-x-3 [&amp;_dl]:mt-1 [&amp;_dl]:mb-0 [&amp;_dl]:mx-0 [&amp;_dd]:m-0 [&amp;_dd]:text-muted-foreground"
          >
            <summary>{{ $t('devices.exactTimes') }}</summary>
            <dl>
              <dt>{{ $t('devices.signedInLabel') }}</dt>
              <dd>{{ exactTime(device.signedInAt) }}</dd>
              <dt>{{ $t('devices.lastSeen') }}</dt>
              <dd>{{ device.lastSeenAt ? exactTime(device.lastSeenAt) : $t('devices.unknown') }}</dd>
              <dt>{{ $t('devices.expires') }}</dt>
              <dd>{{ exactTime(device.expiresAt) }}</dd>
            </dl>
          </details>
          <UiAlert
            v-if="errors[device.id]"
            as="small"
            variant="destructive"
            presentation="inline"
            class="form-error mt-3"
            >{{ errors[device.id] }}</UiAlert
          ></span
        >
        <UiButton
          size="sm"
          :variant="device.current ? 'secondary' : 'destructive'"
          :disabled="busy.has(device.id)"
          :aria-label="
            device.current
              ? $t('devices.signOutThisAria')
              : $t('devices.signOutNamedAria', { name: deviceName(device) })
          "
          @click="signOut(device)"
        >
          {{ busy.has(device.id) ? $t('devices.signingOut') : $t('common.signOut') }}
        </UiButton>
      </li>
    </ul>
    <FieldHint
      v-if="notice"
      role="status"
    >
      {{ notice }}
    </FieldHint>
  </section>
</template>
