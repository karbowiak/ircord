<script setup vapor lang="ts">
import { errorText } from '../../utils/api'
/** Settings → Devices: where this account is signed in. */
import { useQuery, useQueryCache } from '@pinia/colada'
import { devicesQuery } from '../../queries/devices'
import {
  deviceName,
  exactTime,
  mobileDevice,
  notificationText,
  presence,
  relativeTime,
  type Device,
  type DeviceTranslate,
} from '../../../shared/utils/devices'

const auth = useAuthStore()
const appearance = useAppearanceStore()
const { confirm } = usePrompt()
const { t, locale } = useI18n()
const $t = t
const queryCache = useQueryCache()
const list = useQuery(devicesQuery)
const chat = useChatStore()
let refreshTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => chat.routingSnapshot?.revision,
  () => {
    if (refreshTimer) return
    refreshTimer = setTimeout(() => {
      refreshTimer = undefined
      void queryCache.invalidateQueries({ key: devicesQuery.key }).catch(() => {})
    }, 1000)
  },
)
onScopeDispose(() => clearTimeout(refreshTimer))
const translateDevice: DeviceTranslate = (key, values) => t(`mediaUi.devices.${key}`, values ?? {})
const localizedDeviceName = (device: Device) => deviceName(device, translateDevice)
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
const notice = ref<{ key: 'devices.signedOut' | 'devices.alreadySignedOut'; name: string } | null>(
  null,
)

async function signOut(device: Device) {
  const name = localizedDeviceName(device)
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
  notice.value = null
  try {
    await apiClient().signOutDevice(device.id)
    if (device.current) return auth.signedOut()
    notice.value = { key: 'devices.signedOut', name }
  } catch (error) {
    if (errorStatus(error) === 404) {
      if (device.current) return auth.signedOut()
      notice.value = { key: 'devices.alreadySignedOut', name }
    } else {
      // The device is still signed in; say so rather than dropping it from the list.
      errors.value = {
        ...errors.value,
        [device.id]: t('devices.signOutFailed', {
          name,
          error: errorText(error, (key) => t(key)),
        }),
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
    data-setting="device-sessions"
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
        {{
          $t('devices.loadFailed', {
            error: errorText(list.error.value, (key) => t(key)),
          })
        }}
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
        :aria-label="
          device.current
            ? $t('devices.currentRow', { name: localizedDeviceName(device) })
            : localizedDeviceName(device)
        "
      >
        <AppIcon
          :name="mobileDevice(device) ? 'smartphone' : 'monitor'"
          :size="20"
        />
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong
            >{{ localizedDeviceName(device)
            }}<UiBadge
              v-if="device.current"
              variant="secondary"
              class="device-badge ml-2 border-0 align-[1px]"
              >{{ $t('devices.thisDevice') }}</UiBadge
            ></strong
          ><small>
            {{ presence(device, now, locale, translateDevice) }}
            ·
            <time
              :datetime="device.signedInAt"
              :title="exactTime(device.signedInAt, undefined, appearance.settings.timeFormat)"
              >{{
                $t('devices.signedIn', {
                  time: relativeTime(device.signedInAt, now, locale, translateDevice),
                })
              }}</time
            >
            · {{ notificationText(device, translateDevice) }}</small
          >
          <small v-if="device.focal">{{ $t('devices.selected') }}</small>
          <small>{{
            $t('devices.browserType', {
              browser:
                [device.browser, device.os].filter(Boolean).join(' · ') || $t('devices.unknown'),
              type: $t(`devices.type.${device.formFactor || 'desktop'}`),
            })
          }}</small>
          <small
            >{{
              $t('devices.connection', {
                state: $t(
                  device.status === 'connected' ? 'devices.connected' : 'devices.disconnected',
                ),
              })
            }}
            · {{ $t(device.active ? 'devices.active' : 'devices.inactive') }}</small
          >
          <small>{{
            $t('devices.lastSeenRelative', {
              time: device.lastSeenAt
                ? relativeTime(device.lastSeenAt, now, locale, translateDevice)
                : $t('devices.unknown'),
            })
          }}</small>
          <small>{{
            $t('devices.lastActiveRelative', {
              time: device.lastActiveAt
                ? relativeTime(device.lastActiveAt, now, locale, translateDevice)
                : $t('devices.unknown'),
            })
          }}</small>
          <details
            class="device-details text-[12px] text-faint [&amp;_summary]:w-fit [&amp;_summary]:cursor-pointer [&amp;_dl]:grid [&amp;_dl]:grid-cols-[auto_1fr] [&amp;_dl]:gap-y-0.5 [&amp;_dl]:gap-x-3 [&amp;_dl]:mt-1 [&amp;_dl]:mb-0 [&amp;_dl]:mx-0 [&amp;_dd]:m-0 [&amp;_dd]:text-muted-foreground"
          >
            <summary>{{ $t('devices.exactTimes') }}</summary>
            <dl>
              <dt>{{ $t('devices.signedInLabel') }}</dt>
              <dd>{{ exactTime(device.signedInAt, undefined, appearance.settings.timeFormat) }}</dd>
              <dt>{{ $t('devices.lastSeen') }}</dt>
              <dd>
                {{
                  device.lastSeenAt
                    ? exactTime(device.lastSeenAt, undefined, appearance.settings.timeFormat)
                    : $t('devices.unknown')
                }}
              </dd>
              <dt>{{ $t('devices.lastActive') }}</dt>
              <dd>
                {{
                  device.lastActiveAt
                    ? exactTime(device.lastActiveAt, undefined, appearance.settings.timeFormat)
                    : $t('devices.unknown')
                }}
              </dd>
              <template v-if="device.notifications">
                <dt>{{ $t('devices.preferences.permission') }}</dt>
                <dd>
                  {{
                    $t(`devices.preferences.permissionValues.${device.notifications.permission}`)
                  }}
                </dd>
                <dt>{{ $t('devices.preferences.subscription') }}</dt>
                <dd>
                  {{
                    $t(
                      device.notifications.subscription
                        ? 'devices.preferences.registered'
                        : 'devices.preferences.notRegistered',
                    )
                  }}
                </dd>
                <dt>{{ $t('devices.preferences.reported') }}</dt>
                <dd>
                  {{
                    device.notifications.capabilityReportedAt
                      ? exactTime(
                          device.notifications.capabilityReportedAt,
                          undefined,
                          appearance.settings.timeFormat,
                        )
                      : $t('devices.unknown')
                  }}
                </dd>
              </template>
              <dt>{{ $t('devices.expires') }}</dt>
              <dd>{{ exactTime(device.expiresAt, undefined, appearance.settings.timeFormat) }}</dd>
            </dl>
          </details>
          <details class="mt-2">
            <summary class="cursor-pointer text-[12px]">
              {{ $t('devices.preferences.heading') }}
            </summary>
            <DeviceNotificationPreferences
              class="mt-3"
              :device="device"
              :delivery="!device.current"
              name
            />
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
              : $t('devices.signOutNamedAria', { name: localizedDeviceName(device) })
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
      {{ $t(notice.key, { name: notice.name }) }}
    </FieldHint>
  </section>
</template>
