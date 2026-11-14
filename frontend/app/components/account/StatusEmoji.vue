<script setup vapor lang="ts">
const props = defineProps<{ value: string }>()
const media = useMediaStore()
const shortcode = computed(() => /^:([\w+-]{1,64}):$/.exec(props.value)?.[1])
watch(
  shortcode,
  (code) => {
    if (code) media.needCustom()
  },
  { immediate: true },
)
const item = computed(() =>
  shortcode.value ? media.byShortcodeAll.get(shortcode.value) : undefined,
)
</script>

<template>
  <CustomEmoji
    v-if="item?.url && !item.unicode"
    :item="item"
    icon
  />
  <span v-else>{{ value }}</span>
</template>
