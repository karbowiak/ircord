<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { administrationAuditQuery, administrationStatusQuery } from '../queries/administration'
import { formatAdminBytes, formatDuration } from '../utils/administration'

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
const count = (n: number) => n.toLocaleString()

const actions: Record<string, string> = {
  'owner-grant': 'made an owner',
  'owner-revoke': 'removed as owner',
}
</script>
<template>
  <div class="admin-page grid gap-2">
    <p
      v-if="status.error.value"
      class="form-error text-destructive text-[13px] mt-3"
      role="alert"
    >
      {{ errorText(status.error.value) }}
    </p>
    <template v-else-if="status.data.value">
      <SettingGroup title="Backend">
        <dl
          class="admin-facts grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5 m-0 [&amp;>div]:px-3 [&amp;>div]:py-2.5 [&amp;>div]:rounded-md [&amp;>div]:bg-field [&amp;>div]:shadow-[inset_0_0_0_1px_var(--border)] [&amp;_dt]:text-muted-foreground [&amp;_dt]:text-[12px] [&amp;_dd]:mt-0.5 [&amp;_dd]:mb-0 [&amp;_dd]:mx-0 [&amp;_dd]:text-[15px] [&amp;_dd]:font-[600] [&amp;_dd]:tabular-nums [&amp;_dd_small]:ml-1 [&amp;_dd_small]:text-faint [&amp;_dd_small]:font-[400] [&amp;_dd_small]:font-code"
        >
          <div>
            <dt>Health</dt>
            <dd>
              <span
                class="admin-state [&amp;.is-ok]:text-success [&amp;.is-degraded]:text-warning"
                :class="`is-${status.data.value.status}`"
                >{{ status.data.value.status === 'ok' ? 'Healthy' : 'Degraded' }}</span
              >
            </dd>
          </div>
          <div>
            <dt>Database</dt>
            <dd>{{ status.data.value.database === 'available' ? 'Available' : 'Unavailable' }}</dd>
          </div>
          <div>
            <dt>Version</dt>
            <dd>
              {{ status.data.value.version }}
              <small v-if="status.data.value.commit">{{
                status.data.value.commit.slice(0, 7)
              }}</small>
            </dd>
          </div>
          <div>
            <dt>Up for</dt>
            <dd :title="full(status.data.value.startedAt)">
              {{ formatDuration(status.data.value.uptimeSeconds) }}
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
            <dt>Active accounts</dt>
            <dd>{{ count(status.data.value.overview.accounts) }}</dd>
          </div>
          <div>
            <dt>Signed-in sessions</dt>
            <dd>{{ count(status.data.value.overview.sessions) }}</dd>
          </div>
          <div>
            <dt>Saved networks</dt>
            <dd>{{ count(status.data.value.overview.networks) }}</dd>
          </div>
          <div>
            <dt>Live IRC connections</dt>
            <dd>{{ count(status.data.value.overview.ircConnections) }}</dd>
          </div>
          <div>
            <dt>Stored messages</dt>
            <dd>{{ count(status.data.value.overview.messages) }}</dd>
          </div>
          <div>
            <dt>Database size</dt>
            <dd>{{ formatAdminBytes(status.data.value.overview.databaseBytes) }}</dd>
          </div>
        </dl>
      </SettingGroup>
      <SettingGroup title="IRC server">
        <p
          v-if="!status.data.value.ergo"
          class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
        >
          This backend isn't associated with an IRC server.
        </p>
        <p
          v-else-if="status.data.value.ergo.state !== 'ok'"
          class="form-error text-destructive text-[13px] mt-3"
        >
          {{ status.data.value.ergo.server }} isn't answering:
          {{ status.data.value.ergo.error ?? 'unavailable' }}
        </p>
        <dl
          v-else-if="status.data.value.ergo.statistics"
          class="admin-facts grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5 m-0 [&amp;>div]:px-3 [&amp;>div]:py-2.5 [&amp;>div]:rounded-md [&amp;>div]:bg-field [&amp;>div]:shadow-[inset_0_0_0_1px_var(--border)] [&amp;_dt]:text-muted-foreground [&amp;_dt]:text-[12px] [&amp;_dd]:mt-0.5 [&amp;_dd]:mb-0 [&amp;_dd]:mx-0 [&amp;_dd]:text-[15px] [&amp;_dd]:font-[600] [&amp;_dd]:tabular-nums [&amp;_dd_small]:ml-1 [&amp;_dd_small]:text-faint [&amp;_dd_small]:font-[400] [&amp;_dd_small]:font-code"
        >
          <div>
            <dt>Server</dt>
            <dd>{{ status.data.value.ergo.server }}</dd>
          </div>
          <div>
            <dt>Version</dt>
            <dd>{{ status.data.value.ergo.statistics.version }}</dd>
          </div>
          <div>
            <dt>Users online</dt>
            <dd>{{ count(status.data.value.ergo.statistics.users.total) }}</dd>
          </div>
          <div>
            <dt>Channels</dt>
            <dd>{{ count(status.data.value.ergo.statistics.channels) }}</dd>
          </div>
          <div>
            <dt>Up for</dt>
            <dd>{{ formatDuration(status.data.value.ergo.statistics.uptimeSeconds) }}</dd>
          </div>
        </dl>
      </SettingGroup>
      <SettingGroup title="Recent actions">
        <p
          v-if="!audit.data.value?.items.length"
          class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
        >
          Nothing done in Administration yet.
        </p>
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
              >{{ new Date(event.at).toLocaleDateString() }}</time
            >
            <span
              ><strong>{{ event.actor }}</strong>
              {{
                event.outcome.startsWith('refused') || event.outcome === 'failed' ? 'tried:' : ''
              }}
              <strong>{{ event.target }}</strong> {{ actions[event.action] ?? event.action
              }}<template v-if="event.outcome.startsWith('refused') || event.outcome === 'failed'">
                ({{ event.outcome }})</template
              ><small v-if="event.reason"> — {{ event.reason }}</small></span
            >
          </li>
        </ul>
      </SettingGroup>
    </template>
    <p
      v-else
      class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
    >
      Loading…
    </p>
  </div>
</template>
