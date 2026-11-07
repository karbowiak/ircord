<script setup lang="ts">
import type { Member } from '../../shared/types/chat'
const props = withDefaults(
  defineProps<{
    member?: Member
    size?: 'tiny' | 'small' | 'normal' | 'large'
    presence?: boolean
  }>(),
  { size: 'normal', presence: false },
)
const imageFailed = ref(false)
watch(
  () => props.member?.avatarUrl,
  () => (imageFailed.value = false),
)
</script>
<template>
  <span
    class="avatar"
    :class="[`avatar--${size}`, `tone-${member?.color || 'lavender'}`]"
    aria-hidden="true"
  >
    <img
      v-if="member?.avatarUrl && !imageFailed"
      :src="member.avatarUrl"
      alt=""
      class="avatar-image"
      decoding="async"
      referrerpolicy="no-referrer"
      @error="imageFailed = true"
    />
    <AppIcon
      v-else-if="member?.role === 'bot'"
      name="brand"
      :size="size === 'large' ? 32 : size === 'normal' ? 18 : 13"
    />
    <span v-else>{{ member?.displayName.slice(0, 1).toUpperCase() || '?' }}</span>
    <span
      v-if="presence"
      class="presence"
      :class="`presence--${member?.status || 'offline'}`"
    />
  </span>
</template>
