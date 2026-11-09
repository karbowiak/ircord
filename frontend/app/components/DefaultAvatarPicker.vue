<script setup lang="ts">
import { DEFAULT_AVATAR_PATH, type DefaultAvatar } from '../../shared/utils/default-avatars'
/**
 * A grid of the default avatars as one radio group (arrow keys move between them). The
 * suggested one, when given, carries a small marker until another is picked.
 */
const props = defineProps<{
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
const { n } = useI18n()
function choose(value: unknown) {
  const avatar = props.avatars.find((item) => item.id === value)
  if (avatar) emit('select', avatar)
}
</script>
<template>
  <UiRadioGroup
    class="default-avatar-picker grid grid-cols-[repeat(auto-fill,minmax(40px,1fr))] gap-2 max-h-66 overflow-y-auto p-1"
    :model-value="selected"
    :name="name"
    :disabled="disabled"
    :aria-label="label"
    @update:model-value="choose"
  >
    <UiRadioGroupItem
      v-for="(avatar, index) in avatars"
      :key="avatar.id"
      variant="swatch"
      class="default-avatar relative block aspect-square rounded-full p-0 outline-2 outline-offset-2 transition-transform duration-150 hover:scale-[1.06] focus-visible:outline-offset-4 motion-reduce:transition-none motion-reduce:hover:scale-100"
      :class="
        avatar.id === selected
          ? 'is-selected outline-primary outline-solid'
          : avatar.id === suggested
            ? 'is-suggested outline-border outline-dashed'
            : 'outline-transparent outline-solid'
      "
      :value="avatar.id"
      :aria-label="
        $t(avatar.id === suggested ? 'mediaUi.avatars.suggested' : 'mediaUi.avatars.default', {
          index: n(index + 1),
        })
      "
      :title="avatar.id === suggested ? $t('mediaUi.avatars.suggestedTitle') : undefined"
    >
      <NuxtImg
        class="size-full rounded-full object-cover"
        :src="DEFAULT_AVATAR_PATH + avatar.file"
        alt=""
        width="160"
        height="160"
        loading="lazy"
        decoding="async"
      />
    </UiRadioGroupItem>
  </UiRadioGroup>
</template>
