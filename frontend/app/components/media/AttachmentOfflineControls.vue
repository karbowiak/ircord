<script setup vapor lang="ts">
const props = defineProps<{
  saved: boolean
  pinned: boolean
  offline: boolean
  available: boolean
  saving: boolean
}>()
defineEmits<{ toggle: [] }>()
const { t } = useI18n()
const action = computed(() =>
  props.saving
    ? t('mediaUi.attachment.saving')
    : props.pinned
      ? t('mediaUi.attachment.stopKeeping')
      : t('mediaUi.attachment.keepOffline'),
)
const description = computed(() =>
  [
    props.saved
      ? props.pinned
        ? t('mediaUi.attachment.keptOffline')
        : t('mediaUi.attachment.availableOffline')
      : props.offline
        ? t('mediaUi.attachment.notSavedHere')
        : '',
    action.value,
  ]
    .filter(Boolean)
    .join(' · '),
)
</script>
<template>
  <div class="flex size-7 items-center justify-center [@media(pointer:coarse)]:size-11">
    <AppTooltip
      v-if="available || saved || pinned"
      :text="description"
      side="top"
    >
      <UiButton
        type="button"
        variant="ghost"
        size="icon-xs"
        class="bg-background/95 shadow-control"
        :class="{ 'text-primary': pinned, 'text-success': saved && !pinned }"
        :aria-label="description"
        :aria-pressed="pinned"
        :aria-busy="saving || undefined"
        :disabled="!available || saving || (offline && !saved && !pinned)"
        @click="$emit('toggle')"
      >
        <AppIcon
          :name="saving ? 'refresh' : saved ? 'cloud' : 'cloudOff'"
          :size="14"
          :class="{ 'animate-spin motion-reduce:animate-none': saving }"
        />
      </UiButton>
    </AppTooltip>
  </div>
</template>
