<script setup vapor lang="ts">
import type { Member } from '../../../shared/types/chat'
import { uploadImageDensities } from '../../../shared/utils/upload-image'
const props = withDefaults(
  defineProps<{
    member?: Pick<Member, 'displayName'> &
      Partial<Pick<Member, 'avatarUrl' | 'color' | 'role' | 'device' | 'status'>>
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
    <span v-else>{{ member?.displayName.slice(0, 1).toUpperCase() || '?' }}</span>
    <!-- Bots (IRC +B or the bot tag) carry a small bot mark at the top right. -->
    <span
      v-if="member?.role === 'bot'"
      class="bot-mark absolute top-[-5px] right-[-5px] inline-flex size-[19px] items-center justify-center rounded-full bg-primary text-primary-foreground [border:2px_solid_var(--sidebar)] data-[size=tiny]:top-[-4px] data-[size=tiny]:right-[-4px] data-[size=tiny]:size-[14px] data-[size=tiny]:[border-width:1.5px] data-[size=small]:size-[17px] data-[size=large]:top-0 data-[size=large]:right-0 data-[size=large]:size-[28px] data-[size=large]:[border-width:3px]"
      :data-size="size"
      ><AppIcon
        name="bot"
        :size="size === 'large' ? 17 : size === 'tiny' ? 9 : size === 'small' ? 11 : 12"
    /></span>
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
