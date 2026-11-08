<script setup lang="ts">
import { useQuery, useQueryCache } from '@pinia/colada'
import { administrationAccessQuery } from '../queries/administration'

/**
 * Administration (#165): a separate panel for IRCord server owners, not a Settings section.
 * Founders and channel operators never see it; the backend refuses every request from anyone
 * who isn't an owner at that moment.
 */
const emit = defineEmits<{ close: [] }>()
const sections = [
  { id: 'status', name: 'Status', icon: 'monitor' },
  { id: 'users', name: 'Users', icon: 'users' },
] as const
type Section = (typeof sections)[number]['id']
const tab = ref<Section>('status')
const current = computed(() => sections.find((s) => s.id === tab.value)!)
const access = useQuery(administrationAccessQuery)
const queryCache = useQueryCache()

// Losing ownership while the panel is open closes it.
watch(
  () => access.data.value?.owner,
  (owner) => owner === false && emit('close'),
)
/** A refused request means ownership is gone: recheck, which closes the panel. */
provide(administrationRefusedKey, () =>
  queryCache.invalidateQueries({ key: administrationAccessQuery.key }),
)
</script>
<template>
  <AppDialog
    title="Administration"
    panel
    @close="emit('close')"
    ><div class="settings-layout admin-layout">
      <nav
        class="settings-nav"
        aria-label="Administration"
      >
        <div class="settings-nav-section">
          <div class="settings-nav-group">Server</div>
          <button
            v-for="item in sections"
            :key="item.id"
            :class="{ active: tab === item.id }"
            :aria-current="tab === item.id ? 'page' : undefined"
            @click="tab = item.id"
          >
            <AppIcon
              :name="item.icon"
              :size="17"
            />{{ item.name }}
          </button>
        </div>
      </nav>
      <div class="settings-content">
        <h3 class="settings-title">{{ current.name }}</h3>
        <AdminStatus v-if="tab === 'status'" />
        <AdminUsers v-else />
      </div></div
  ></AppDialog>
</template>
