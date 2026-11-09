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
        offlineStatus.messages
          ? 'Searching saved messages from the last seven days and messages kept offline.'
          : 'No message archive is available on this device yet.'
      }}<span v-if="offlineStatus.phase === 'partial'">
        Some history has not finished downloading.</span
      ></span
    >
  </p>
</template>
