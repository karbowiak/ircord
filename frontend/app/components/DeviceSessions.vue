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

const busy = ref('')
const errors = ref<Record<string, string>>({})
const notice = ref('')
const status = (error: unknown) => (error as { statusCode?: number } | null)?.statusCode

async function signOut(device: Device) {
  const name = deviceName(device)
  const ok = await confirm(
    device.current
      ? {
          title: 'Sign out this device?',
          body: 'This is the device you’re using. Every IRCord tab in this browser signs out and you return to the sign-in screen.',
          confirm: 'Sign out',
          danger: true,
        }
      : {
          title: `Sign out ${name}?`,
          body: 'Every IRCord tab on that device signs out, it stops getting this account’s notifications, and it has to sign in again with your username and password. This device, your other devices and your account stay as they are.',
          confirm: 'Sign out device',
          danger: true,
        },
  )
  if (!ok) return
  busy.value = device.id
  errors.value = { ...errors.value, [device.id]: '' }
  notice.value = ''
  try {
    await apiClient().signOutDevice(device.id)
    if (device.current) return auth.signedOut()
    notice.value = `Signed out ${name}.`
  } catch (error) {
    if (status(error) === 404) {
      if (device.current) return auth.signedOut()
      notice.value = `${name} was already signed out.`
    } else {
      // The device is still signed in; say so rather than dropping it from the list.
      errors.value = {
        ...errors.value,
        [device.id]: `Couldn’t sign out ${name}: ${errorText(error)}`,
      }
    }
  } finally {
    busy.value = ''
    void queryCache.invalidateQueries({ key: devicesQuery.key }).catch(() => {})
  }
}
</script>
<template>
  <section
    class="setting-group"
    aria-labelledby="devices-heading"
  >
    <h3 id="devices-heading">Devices &amp; sessions</h3>
    <p class="field-hint">
      Where this account is signed in. Tabs in one browser share a sign-in; another browser or
      profile is a separate device.
    </p>
    <p
      v-if="list.isPending.value"
      class="field-hint"
      role="status"
    >
      Loading devices…
    </p>
    <div
      v-else-if="list.error.value && !list.data.value"
      class="setting-row"
    >
      <p
        class="form-error"
        role="alert"
      >
        Couldn’t load your devices: {{ errorText(list.error.value) }}
      </p>
      <button
        class="button secondary small"
        @click="list.refetch()"
      >
        Try again
      </button>
    </div>
    <p
      v-else-if="!devices.length"
      class="field-hint"
    >
      No signed-in devices are recorded.
    </p>
    <ul
      v-else
      class="device-list"
      aria-label="Signed-in devices"
    >
      <li
        v-for="device in devices"
        :key="device.id"
        class="device-row"
        :aria-label="deviceName(device) + (device.current ? ', this device' : '')"
      >
        <AppIcon
          :name="mobileDevice(device) ? 'smartphone' : 'monitor'"
          :size="20"
        />
        <span class="setting-copy"
          ><strong
            >{{ deviceName(device)
            }}<span
              v-if="device.current"
              class="device-badge"
              >This device</span
            ></strong
          ><small>
            <span :title="device.lastSeenAt ? exactTime(device.lastSeenAt) : undefined">{{
              presence(device, now)
            }}</span>
            ·
            <span :title="exactTime(device.signedInAt)"
              >Signed in {{ relativeTime(device.signedInAt, now) }}</span
            >
            · {{ notificationText(device, notifications.mode) }}</small
          ><small
            v-if="errors[device.id]"
            class="form-error"
            role="alert"
            >{{ errors[device.id] }}</small
          ></span
        >
        <button
          class="button small"
          :class="device.current ? 'secondary' : 'danger'"
          :disabled="busy === device.id"
          :aria-label="`Sign out ${device.current ? 'this device' : deviceName(device)}`"
          @click="signOut(device)"
        >
          {{ busy === device.id ? 'Signing out…' : 'Sign out' }}
        </button>
      </li>
    </ul>
    <p
      v-if="notice"
      class="field-hint"
      role="status"
    >
      {{ notice }}
    </p>
  </section>
</template>
<style scoped>
.device-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 4px 0 0;
  padding: 0;
  list-style: none;
}
.device-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius);
  background: var(--input);
  color: var(--muted);
}
.device-badge {
  margin-left: 8px;
  padding: 1px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  color: var(--text);
  font-size: 11px;
  font-weight: 600;
  vertical-align: 1px;
}
@media (max-width: 640px) {
  .device-row {
    flex-wrap: wrap;
  }
  .device-row .button {
    margin-left: 32px;
  }
}
</style>
