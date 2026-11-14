<script setup vapor lang="ts">
defineProps<{ rows: { label: string; value: string; help?: string }[] }>()
</script>
<template>
  <dl class="stat-tiles">
    <div
      v-for="row in rows"
      :key="row.label"
      :title="row.help"
    >
      <dt>{{ row.label }}</dt>
      <dd>{{ row.value }}</dd>
    </div>
  </dl>
</template>
