<script setup lang="ts">
import { offlineStatus } from '../utils/offline-archive'
defineProps<{ offline?: boolean }>()
</script>
<template>
  <p
    v-if="offline || offlineStatus.offline"
    class="offline-search-notice flex items-center gap-2 px-3 py-2 text-[length:var(--ui-caption)] text-muted-foreground"
    role="status"
  >
    <AppIcon
      name="offline"
      :size="13"
    />
    <span
      >{{
        offlineStatus.messages ? $t('offlineArchive.searching') : $t('offlineArchive.unavailable')
      }}<span v-if="offlineStatus.phase === 'partial'">
        {{ $t('offlineArchive.partial') }}</span
      ></span
    >
  </p>
</template>
