<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { administrationAuditQuery, administrationStatusQuery } from '../queries/administration'
import { formatBytes, formatDuration } from '../utils/administration'

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
  <div class="admin-page">
    <p
      v-if="status.error.value"
      class="form-error"
      role="alert"
    >
      {{ errorText(status.error.value) }}
    </p>
    <template v-else-if="status.data.value">
      <SettingGroup title="Backend">
        <dl class="admin-facts">
          <div>
            <dt>Health</dt>
            <dd>
              <span
                class="admin-state"
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
        <dl class="admin-facts">
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
            <dd>{{ formatBytes(status.data.value.overview.databaseBytes) }}</dd>
          </div>
        </dl>
      </SettingGroup>
      <SettingGroup title="IRC server">
        <p
          v-if="!status.data.value.ergo"
          class="settings-description"
        >
          This backend isn't associated with an IRC server.
        </p>
        <p
          v-else-if="status.data.value.ergo.state !== 'ok'"
          class="form-error"
        >
          {{ status.data.value.ergo.server }} isn't answering:
          {{ status.data.value.ergo.error ?? 'unavailable' }}
        </p>
        <dl
          v-else-if="status.data.value.ergo.statistics"
          class="admin-facts"
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
          class="settings-description"
        >
          Nothing done in Administration yet.
        </p>
        <ul
          v-else
          class="admin-audit"
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
      class="settings-description"
    >
      Loading…
    </p>
  </div>
</template>
