<script setup vapor lang="ts">
import { useQuery, useQueryCache } from '@pinia/colada'
import { administrationAccessQuery } from '../queries/administration'

/**
 * Administration (#165): a separate panel for IRCord server owners, not a Settings section.
 * Founders and channel operators never see it; the backend refuses every request from anyone
 * who isn't an owner at that moment.
 */
const emit = defineEmits<{ close: [] }>()
const sections = [
  { id: 'status', name: 'administration.status', icon: 'monitor' },
  { id: 'users', name: 'administration.users', icon: 'users' },
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
const { t: $t } = useI18n()
</script>
<template>
  <AppDialog
    :title="$t('administration.title')"
    panel
    @close="emit('close')"
    ><SettingsLayout
      :label="$t('administration.title')"
      class="admin-layout"
    >
      <template #nav>
        <div class="settings-nav-section contents">
          <div
            class="settings-nav-group pt-0.5 pr-2.5 pb-1 pl-2.5 text-[11px] font-[650] tracking-[0.05em] uppercase text-faint max-[700px]:sr-only"
          >
            {{ $t('administration.server') }}
          </div>
          <SettingsNavItem
            v-for="item in sections"
            :key="item.id"
            :icon="item.icon"
            :active="tab === item.id"
            @click="tab = item.id"
            >{{ $t(item.name) }}</SettingsNavItem
          >
        </div>
      </template>
      <div
        class="settings-content [grid-area:content] pt-5 pr-7 pb-7 pl-7 overflow-y-auto max-[700px]:p-4"
      >
        <SettingsTitle>
          {{ $t(current.name) }}
        </SettingsTitle>
        <AdminStatus v-if="tab === 'status'" />
        <AdminUsers v-else /></div></SettingsLayout
  ></AppDialog>
</template>
