<script setup lang="ts">
import { DEFAULT_AVATAR_PATH, type DefaultAvatar } from '../../shared/utils/default-avatars'
/**
 * A grid of the default avatars as one radio group (arrow keys move between them). The
 * suggested one, when given, carries a small marker until another is picked.
 */
defineProps<{
  avatars: readonly DefaultAvatar[]
  /** The id of the avatar currently chosen, if it is a default. */
  selected?: string
  /** The id of the username-based suggestion. */
  suggested?: string
  /** Radio group name; unique per form. */
  name: string
  label: string
  disabled?: boolean
}>()
const emit = defineEmits<{ select: [avatar: DefaultAvatar] }>()
</script>
<template>
  <div
    class="default-avatar-picker"
    role="radiogroup"
    :aria-label="label"
  >
    <label
      v-for="(avatar, index) in avatars"
      :key="avatar.id"
      class="default-avatar"
      :class="{ 'is-selected': avatar.id === selected, 'is-suggested': avatar.id === suggested }"
      :title="avatar.id === suggested ? 'Suggested for your username' : undefined"
    >
      <input
        class="sr-only"
        type="radio"
        :name="name"
        :value="avatar.id"
        :checked="avatar.id === selected"
        :disabled="disabled"
        :aria-label="`Default avatar ${index + 1}${avatar.id === suggested ? ' (suggested)' : ''}`"
        @change="emit('select', avatar)"
      /><img
        :src="DEFAULT_AVATAR_PATH + avatar.file"
        alt=""
        width="160"
        height="160"
        loading="lazy"
        decoding="async"
      />
    </label>
  </div>
</template>
