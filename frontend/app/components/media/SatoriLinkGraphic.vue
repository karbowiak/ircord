<script setup vapor lang="ts">
import { fallbackLinkSvgDataUrl, renderSatoriLinkSvgDataUrl } from '../../utils/satori-card'

const props = defineProps<{
  url: string
  site?: string
  title?: string
  description?: string
}>()

const dataUrl = ref(fallbackLinkSvgDataUrl(props))
let requestToken = 0
let disposed = false

onBeforeUnmount(() => {
  disposed = true
})

const backgroundStyle = computed(() => ({
  backgroundImage: `url('${dataUrl.value}')`,
}))

async function refresh() {
  const token = ++requestToken
  const snapshot = {
    url: props.url,
    site: props.site,
    title: props.title,
    description: props.description,
  }
  dataUrl.value = fallbackLinkSvgDataUrl(snapshot)
  const rendered = await renderSatoriLinkSvgDataUrl(snapshot)
  if (!disposed && token === requestToken) dataUrl.value = rendered
}

watch(
  () => [props.url, props.site, props.title, props.description],
  () => {
    void refresh()
  },
  { immediate: true },
)
</script>

<template>
  <div
    class="satori-link-graphic size-full bg-cover bg-center bg-no-repeat"
    :style="backgroundStyle"
    aria-hidden="true"
  />
</template>
