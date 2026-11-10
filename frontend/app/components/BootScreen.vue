<script setup lang="ts" vapor>
const { t: $t } = useI18n()
/**
 * The full-screen state before the app can show anything: the brand and a loading bar, or, with
 * a `title`, what went wrong and a Try again button.
 */
defineProps<{ title?: string; detail?: string }>()
const emit = defineEmits<{ retry: [] }>()
</script>
<template>
  <div
    class="boot-screen fixed inset-0 flex flex-col items-center justify-center gap-4 bg-background p-6 text-center text-body"
  >
    <template v-if="title">
      <AppIcon
        name="offline"
        :size="30"
      />
      <h1 class="text-[20px] text-foreground">{{ title }}</h1>
      <p class="max-w-90 text-muted-foreground">{{ detail }}</p>
      <UiButton @click="emit('retry')">
        <AppIcon
          name="refresh"
          :size="16"
        />{{ $t('common.tryAgain') }}
      </UiButton>
    </template>
    <template v-else>
      <BrandMark class="brand-mark block h-12 w-12 flex-none" />
      <div class="loading-track h-[3px] w-30 overflow-hidden rounded-[3px] bg-border">
        <span class="block h-full w-[45%] animate-[loading_1.2s_ease-in-out_infinite] bg-primary" />
      </div>
    </template>
  </div>
</template>
