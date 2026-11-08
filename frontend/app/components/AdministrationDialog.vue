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
    ><div
      class="settings-layout flex-1 min-h-0 grid [grid-template-columns:200px_minmax(0,_1fr)] [grid-template-rows:auto_minmax(0,_1fr)] [grid-template-areas:'search_content'_'nav_content'] admin-layout"
    >
      <nav
        class="settings-nav [grid-area:nav] flex flex-col gap-0.5 py-3 px-2 bg-sidebar overflow-y-auto"
        aria-label="Administration"
      >
        <div class="settings-nav-section contents">
          <div
            class="settings-nav-group pt-3.5 pr-2.5 pb-1 pl-2.5 text-[11px] font-[650] tracking-[0.05em] uppercase text-faint"
          >
            Server
          </div>
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
      <div class="settings-content pt-5 pr-7 pb-7 pl-7 overflow-y-auto">
        <h3 class="settings-title text-[20px] font-[700] tracking-[-0.01em] mb-4">
          {{ current.name }}
        </h3>
        <AdminStatus v-if="tab === 'status'" />
        <AdminUsers v-else />
      </div></div
  ></AppDialog>
</template>
