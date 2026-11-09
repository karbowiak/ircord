<script setup lang="ts">
import WorkspaceLayout from './layouts/default.vue'
</script>
<template>
  <!-- One delay for every tooltip, so moving along a toolbar shows the next label at once. -->
  <UiTooltipProvider>
    <WorkspaceLayout><NuxtPage /></WorkspaceLayout>
  </UiTooltipProvider>
</template>
