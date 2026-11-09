<script setup lang="ts">
const { locale } = useI18n()
useHead(() => ({ htmlAttrs: { lang: locale.value } }))
</script>

<template>
  <!-- One delay for every tooltip, so moving along a toolbar shows the next label at once. -->
  <UiTooltipProvider>
    <NuxtLayout name="default"><NuxtPage /></NuxtLayout>
  </UiTooltipProvider>
  <!-- One persistent live region for useAnnouncer(): status text that appears with its region
       is often not read, so notices announce here instead of mounting their own. -->
  <NuxtAnnouncer />
</template>
