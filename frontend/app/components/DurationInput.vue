<script setup lang="ts">
/** A duration in whole seconds, entered as a number and a unit (seconds, minutes or hours). */
const props = defineProps<{ modelValue: number; max: number; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [seconds: number] }>()
const units = [
  { value: 1, label: 'seconds' },
  { value: 60, label: 'minutes' },
  { value: 3600, label: 'hours' },
] as const
type Unit = (typeof units)[number]['value']
/** The largest unit that shows the value exactly; zero shows in the last chosen unit. */
const chosen = ref<Unit>(1)
const unit = computed<Unit>(() => {
  if (props.modelValue === 0) return chosen.value
  return [...units].reverse().find((u) => props.modelValue % u.value === 0)?.value ?? 1
})
const amount = computed(() => props.modelValue / unit.value)
function set(value: number, next: Unit = unit.value) {
  chosen.value = next
  if (!Number.isFinite(value) || value < 0) value = 0
  emit('update:modelValue', Math.min(props.max, Math.round(value * next)))
}
</script>
<template>
  <span class="duration-input inline-flex items-center gap-1.5">
    <UiInput
      type="number"
      class="w-[5.5em]"
      inputmode="numeric"
      min="0"
      :max="Math.floor(max / unit)"
      :model-value="amount"
      :aria-label="label"
      @change="set(Number(($event.target as HTMLInputElement).value))"
    />
    <AppSelect
      :model-value="unit"
      :options="units.filter((u) => u.value <= max)"
      :aria-label="`${label} unit`"
      @change="set(amount, $event)"
    />
  </span>
</template>
