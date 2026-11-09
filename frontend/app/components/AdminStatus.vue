<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { administrationAuditQuery, administrationStatusQuery } from '../queries/administration'

/**
 * Administration → Status: is the backend healthy, what is it running, and how much is it
 * carrying. IRCord's own counts (accounts, sessions, stored messages) are kept apart from the
 * associated IRC server's (users and channels on the network).
 */
const status = useQuery(administrationStatusQuery)
const audit = useQuery(administrationAuditQuery)
const refused = useAdministrationRefusal()
watch(() => status.error.value, refused)
const { full } = useTimeFormat()
const { n, locale, t } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const count = (value: number) => n(value)
const formatBytes = (bytes: number) => {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  const maximumFractionDigits = value < 10 && unit ? 1 : 0
  return `${n(value, { maximumFractionDigits })} ${units[unit]}`
}
const formatUptime = (seconds: number) => {
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return [
    days ? t('collectionsUi.serverOverview.days', { count: n(days) }, days) : '',
    hours ? t('collectionsUi.serverOverview.hours', { count: n(hours) }, hours) : '',
    t('collectionsUi.serverOverview.minutes', { count: n(minutes) }, minutes),
  ]
    .filter(Boolean)
    .join(' ')
}

const actions: Record<string, string> = {
  'owner-grant': 'administration.madeOwner',
  'owner-revoke': 'administration.removedOwner',
}
</script>
<template>
  <div class="admin-page grid gap-2">
    <UiAlert
      v-if="status.error.value"
      as="p"
      variant="destructive"
      presentation="inline"
      class="form-error mt-3"
    >
      {{ errorMessage(status.error.value) }}
    </UiAlert>
    <template v-else-if="status.data.value">
      <SettingGroup :title="$t('administration.backend')">
        <dl
          class="admin-facts grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5 m-0 [&amp;>div]:px-3 [&amp;>div]:py-2.5 [&amp;>div]:rounded-md [&amp;>div]:bg-field [&amp;>div]:shadow-[inset_0_0_0_1px_var(--border)] [&amp;_dt]:text-muted-foreground [&amp;_dt]:text-[12px] [&amp;_dd]:mt-0.5 [&amp;_dd]:mb-0 [&amp;_dd]:mx-0 [&amp;_dd]:text-[15px] [&amp;_dd]:font-[600] [&amp;_dd]:tabular-nums [&amp;_dd_small]:ml-1 [&amp;_dd_small]:text-faint [&amp;_dd_small]:font-[400] [&amp;_dd_small]:font-code"
        >
          <div>
            <dt>{{ $t('administration.health') }}</dt>
            <dd>
              <span
                class="admin-state [&amp;.is-ok]:text-success [&amp;.is-degraded]:text-warning"
                :class="`is-${status.data.value.status}`"
                >{{
                  status.data.value.status === 'ok'
                    ? $t('administration.healthy')
                    : $t('administration.degraded')
                }}</span
              >
            </dd>
          </div>
          <div>
            <dt>{{ $t('administration.database') }}</dt>
            <dd>
              {{
                status.data.value.database === 'available'
                  ? $t('administration.available')
                  : $t('administration.unavailable')
              }}
            </dd>
          </div>
          <div>
            <dt>{{ $t('administration.version') }}</dt>
            <dd>
              {{ status.data.value.version }}
              <small v-if="status.data.value.commit">{{
                status.data.value.commit.slice(0, 7)
              }}</small>
            </dd>
          </div>
          <div>
            <dt>{{ $t('administration.uptime') }}</dt>
            <dd :title="full(status.data.value.startedAt)">
              {{ formatUptime(status.data.value.uptimeSeconds) }}
            </dd>
          </div>
        </dl>
      </SettingGroup>
      <SettingGroup
        v-if="status.data.value.overview"
        title="IRCord"
      >
        <dl
          class="admin-facts grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5 m-0 [&amp;>div]:px-3 [&amp;>div]:py-2.5 [&amp;>div]:rounded-md [&amp;>div]:bg-field [&amp;>div]:shadow-[inset_0_0_0_1px_var(--border)] [&amp;_dt]:text-muted-foreground [&amp;_dt]:text-[12px] [&amp;_dd]:mt-0.5 [&amp;_dd]:mb-0 [&amp;_dd]:mx-0 [&amp;_dd]:text-[15px] [&amp;_dd]:font-[600] [&amp;_dd]:tabular-nums [&amp;_dd_small]:ml-1 [&amp;_dd_small]:text-faint [&amp;_dd_small]:font-[400] [&amp;_dd_small]:font-code"
        >
          <div>
            <dt>{{ $t('administration.activeAccounts') }}</dt>
            <dd>{{ count(status.data.value.overview.accounts) }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.sessions') }}</dt>
            <dd>{{ count(status.data.value.overview.sessions) }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.networks') }}</dt>
            <dd>{{ count(status.data.value.overview.networks) }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.connections') }}</dt>
            <dd>{{ count(status.data.value.overview.ircConnections) }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.messages') }}</dt>
            <dd>{{ count(status.data.value.overview.messages) }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.databaseSize') }}</dt>
            <dd>{{ formatBytes(status.data.value.overview.databaseBytes) }}</dd>
          </div>
        </dl>
      </SettingGroup>
      <SettingGroup :title="$t('administration.ircServer')">
        <SettingsDescription v-if="!status.data.value.ergo">
          {{ $t('administration.noIrcServer') }}
        </SettingsDescription>
        <p
          v-else-if="status.data.value.ergo.state !== 'ok'"
          class="form-error text-destructive text-[13px] mt-3"
        >
          {{
            $t('administration.serverNotResponding', {
              server: status.data.value.ergo.server,
              error: status.data.value.ergo.error ?? $t('administration.unavailableLower'),
            })
          }}
        </p>
        <dl
          v-else-if="status.data.value.ergo.statistics"
          class="admin-facts grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5 m-0 [&amp;>div]:px-3 [&amp;>div]:py-2.5 [&amp;>div]:rounded-md [&amp;>div]:bg-field [&amp;>div]:shadow-[inset_0_0_0_1px_var(--border)] [&amp;_dt]:text-muted-foreground [&amp;_dt]:text-[12px] [&amp;_dd]:mt-0.5 [&amp;_dd]:mb-0 [&amp;_dd]:mx-0 [&amp;_dd]:text-[15px] [&amp;_dd]:font-[600] [&amp;_dd]:tabular-nums [&amp;_dd_small]:ml-1 [&amp;_dd_small]:text-faint [&amp;_dd_small]:font-[400] [&amp;_dd_small]:font-code"
        >
          <div>
            <dt>{{ $t('administration.server') }}</dt>
            <dd>{{ status.data.value.ergo.server }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.version') }}</dt>
            <dd>{{ status.data.value.ergo.statistics.version }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.usersOnline') }}</dt>
            <dd>{{ count(status.data.value.ergo.statistics.users.total) }}</dd>
          </div>
          <div>
            <dt>{{ $t('conversation.channels') }}</dt>
            <dd>{{ count(status.data.value.ergo.statistics.channels) }}</dd>
          </div>
          <div>
            <dt>{{ $t('administration.uptime') }}</dt>
            <dd>{{ formatUptime(status.data.value.ergo.statistics.uptimeSeconds) }}</dd>
          </div>
        </dl>
      </SettingGroup>
      <SettingGroup :title="$t('administration.recentActions')">
        <SettingsDescription v-if="!audit.data.value?.items.length">
          {{ $t('administration.noActions') }}
        </SettingsDescription>
        <ul
          v-else
          class="admin-audit list-none m-0 p-0 grid gap-1.5 text-[13px] [&amp;_li]:flex [&amp;_li]:gap-2.5 [&amp;_time]:shrink-0 [&amp;_time]:w-[10ch] [&amp;_time]:text-faint [&amp;_time]:tabular-nums [&amp;_small]:text-muted-foreground"
        >
          <li
            v-for="(event, index) in audit.data.value.items"
            :key="index"
          >
            <time
              :datetime="event.at"
              :title="full(event.at)"
              >{{ new Date(event.at).toLocaleDateString(locale) }}</time
            >
            <span
              ><i18n-t
                :keypath="
                  event.outcome.startsWith('refused') || event.outcome === 'failed'
                    ? 'administration.auditFailed'
                    : 'administration.auditSuccess'
                "
                tag="span"
              >
                <template #actor
                  ><strong>{{ event.actor }}</strong></template
                >
                <template #target
                  ><strong>{{ event.target }}</strong></template
                >
                <template #action>{{
                  actions[event.action] ? $t(actions[event.action]!) : event.action
                }}</template>
                <template #outcome>{{ event.outcome }}</template> </i18n-t
              ><small v-if="event.reason"> — {{ event.reason }}</small></span
            >
          </li>
        </ul>
      </SettingGroup>
    </template>
    <SettingsDescription v-else> {{ $t('common.loading') }} </SettingsDescription>
  </div>
</template>
