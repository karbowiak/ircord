<script setup vapor lang="ts">
defineProps<{ rows: { label: string; value: string; help?: string }[] }>()
</script>
<template>
  <dl class="m-0 grid grid-cols-2 gap-x-8 max-[700px]:grid-cols-1">
    <div
      v-for="row in rows"
      :key="row.label"
      class="flex min-w-0 items-baseline justify-between gap-4 border-b border-border/50 py-3"
    >
      <dt
        class="min-w-0 text-[13px] text-muted-foreground"
        :title="row.help"
      >
        {{ row.label }}
      </dt>
      <dd
        class="m-0 max-w-[60%] shrink-0 text-right text-[14px] font-medium tabular-nums [overflow-wrap:anywhere]"
      >
        {{ row.value }}
      </dd>
    </div>
  </dl>
</template>
