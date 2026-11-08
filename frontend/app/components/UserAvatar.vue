<script setup lang="ts">
import type { Member } from '../../shared/types/chat'
const props = withDefaults(
  defineProps<{
    member?: Member
    size?: 'tiny' | 'small' | 'normal' | 'large'
    presence?: boolean
    /** /attract: a short sparkle around the avatar. */
    attract?: boolean
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
    :class="[`avatar--${size}`, `tone-${member?.color || 'lavender'}`, { 'is-attract': attract }]"
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
    <!-- On a phone or tablet (#136) the presence dot becomes that device, Discord-style. -->
    <span
      v-if="presence && member?.device && member.status !== 'offline'"
      class="presence presence--device"
      :class="`presence--${member.status}`"
      :title="member.device === 'phone' ? 'On a phone' : 'On a tablet'"
      ><AppIcon
        :name="member.device === 'phone' ? 'smartphone' : 'tablet'"
        :size="size === 'large' ? 16 : size === 'tiny' ? 9 : 11"
    /></span>
    <span
      v-else-if="presence"
      class="presence"
      :class="`presence--${member?.status || 'offline'}`"
    />
  </span>
</template>
