<script setup lang="ts" vapor>
import type { Member } from '../../../shared/types/chat'
import { memberStatusLabel } from '../../utils/member-status'

const props = defineProps<{ member?: Member; now: number }>()
const label = computed(() => memberStatusLabel(props.member, props.now))
</script>

<template>
  <span
    v-if="label && member"
    data-member-status
    class="flex h-3 min-w-0 max-w-full items-center gap-1 overflow-hidden text-muted-foreground"
  >
    <span
      class="inline-flex h-3 max-w-8 shrink-0 items-center overflow-hidden text-[12px] leading-none [&_.icon-custom-emoji]:size-3"
      aria-hidden="true"
    >
      <StatusEmoji
        v-if="member.statusEmoji"
        :value="member.statusEmoji"
      />
      <AppIcon
        v-else
        name="message"
        :size="12"
      />
    </span>
    <span
      v-if="member.statusText"
      class="min-w-0 truncate text-[11px] leading-3 font-normal tracking-normal"
      aria-hidden="true"
      >{{ member.statusText }}</span
    >
    <span class="sr-only">, {{ label }}</span>
  </span>
</template>
