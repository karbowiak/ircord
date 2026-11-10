<script setup vapor lang="ts">
import type { Member } from '../../../shared/types/chat'
import { uploadImageDensities } from '../../../shared/utils/upload-image'
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
const { t } = useI18n()
const imageFailed = ref(false)
watch(
  () => props.member?.avatarUrl,
  () => (imageFailed.value = false),
)
</script>
<template>
  <span
    class="avatar relative w-9 h-9 rounded-full inline-flex items-center justify-center shrink-0 text-[length:calc(15px_*_var(--interface-scale,1))] font-[600] leading-[1] data-[size=tiny]:size-5.5 data-[size=tiny]:text-[length:calc(11px_*_var(--interface-scale,1))] data-[size=small]:size-7 data-[size=small]:text-[length:calc(12px_*_var(--interface-scale,1))] data-[size=large]:size-20 data-[size=large]:text-[length:calc(30px_*_var(--interface-scale,1))]"
    :data-size="size"
    :class="[`avatar--${size}`, `tone-${member?.color || 'lavender'}`, { 'is-attract': attract }]"
    aria-hidden="true"
  >
    <NuxtImg
      v-if="member?.avatarUrl && !imageFailed"
      :src="member.avatarUrl"
      :width="size === 'large' ? 80 : size === 'normal' ? 36 : size === 'small' ? 28 : 22"
      :height="size === 'large' ? 80 : size === 'normal' ? 36 : size === 'small' ? 28 : 22"
      :densities="uploadImageDensities(member.avatarUrl)"
      alt=""
      class="avatar-image w-full h-full object-cover rounded-[inherit]"
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
      class="presence absolute w-[11px] h-[11px] bottom-[-2px] right-[-2px] [border:2.5px_solid_var(--sidebar)] rounded-full bg-success presence--device"
      :class="`presence--${member.status}`"
      :title="
        member.device === 'phone' ? t('workspaceUi.device.phone') : t('workspaceUi.device.tablet')
      "
      ><AppIcon
        :name="member.device === 'phone' ? 'smartphone' : 'tablet'"
        :size="size === 'large' ? 16 : size === 'tiny' ? 9 : 11"
    /></span>
    <span
      v-else-if="presence"
      class="presence absolute w-[11px] h-[11px] bottom-[-2px] right-[-2px] [border:2.5px_solid_var(--sidebar)] rounded-full bg-success"
      :class="`presence--${member?.status || 'offline'}`"
    />
  </span>
</template>
