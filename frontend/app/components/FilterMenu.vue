<script lang="ts"></script>
<script setup lang="ts">
import {
  ListboxContent,
  ListboxFilter,
  ListboxGroup,
  ListboxGroupLabel,
  ListboxItem,
  ListboxItemIndicator,
  ListboxRoot,
} from 'reka-ui'
import { selectTriggerClass } from './ui/select'
export interface FilterOption {
  value: string
  label: string
  icon?: string
  group?: string
  disabled?: boolean
}

/**
 * A filter dropdown: a select-like button opening a list to pick from, with a search box when the
 * list is long. A string model picks one option and closes; an array model ticks several and stays
 * open, with a way to clear them. Options may be grouped (channels by network).
 */
const props = withDefaults(
  defineProps<{
    options: FilterOption[]
    /** The trigger's text for the current choice. */
    display: string
    label: string
    searchable?: boolean
    searchPlaceholder?: string
    /** Clears an array model ("Any channel"). */
    clearLabel?: string
    note?: string
    /** Fill the row, as a form field does. */
    block?: boolean
  }>(),
  {
    searchable: false,
    searchPlaceholder: undefined,
    clearLabel: undefined,
    note: undefined,
    block: false,
  },
)
const { t } = useI18n()
const model = defineModel<string | string[]>({ required: true })
const open = ref(false)
const query = ref('')
watch(open, (value) => value || (query.value = ''))
const multiple = computed(() => Array.isArray(model.value))
const selection = computed({
  get: () => model.value,
  set: (value: string | string[] | undefined) => {
    if (multiple.value) model.value = (value as string[] | undefined) ?? []
    else {
      model.value = (value as string | undefined) ?? ''
      open.value = false
    }
  },
})
const active = computed(() => (multiple.value ? model.value.length > 0 : model.value !== ''))
const shown = computed(() => {
  const q = query.value.trim().toLowerCase().replace(/^#/, '')
  return q ? props.options.filter((o) => o.label.toLowerCase().includes(q)) : props.options
})
const groups = computed(() => {
  const out: { name: string; options: FilterOption[] }[] = []
  for (const option of shown.value) {
    const name = option.group ?? ''
    const group = out.find((g) => g.name === name)
    if (group) group.options.push(option)
    else out.push({ name, options: [option] })
  }
  return out
})
const itemClass =
  'flex min-h-8 cursor-default select-none items-center gap-2 rounded-sm px-2 py-1 text-[length:var(--ui-small)] text-body outline-none data-highlighted:bg-accent data-highlighted:text-foreground data-disabled:opacity-45 [@media(pointer:coarse)]:min-h-[44px]'
</script>
<template>
  <UiPopover v-model:open="open">
    <UiPopoverTrigger as-child>
      <button
        type="button"
        :class="
          cn(selectTriggerClass, 'filter-menu', block ? 'w-full' : 'w-auto min-w-36 max-w-60')
        "
        :aria-label="t('collectionsUi.filterMenu.triggerLabel', { label, display })"
      >
        <span
          class="min-w-0 flex-1 truncate"
          :class="{ 'text-foreground': active }"
          >{{ display }}</span
        >
        <AppIcon
          name="down"
          :size="15"
          class="shrink-0 text-muted-foreground transition-transform group-data-[state=open]:rotate-180"
        />
      </button>
    </UiPopoverTrigger>
    <UiPopoverContent
      class="p-0"
      :class="block ? 'w-(--reka-popover-trigger-width)' : 'w-64'"
      :aria-label="label"
    >
      <ListboxRoot
        v-model="selection"
        :multiple="multiple"
        :selection-behavior="multiple ? 'toggle' : 'replace'"
        highlight-on-hover
        class="flex max-h-[min(360px,60dvh)] flex-col"
      >
        <div
          v-if="searchable"
          class="search-field-control flex shrink-0 items-center gap-2 border-b [border-bottom-style:solid] border-border/60 px-2.5 text-muted-foreground"
        >
          <AppIcon
            name="search"
            :size="14"
          />
          <ListboxFilter
            v-model="query"
            auto-focus
            class="h-9 min-w-0 flex-1 border-0 bg-transparent text-[length:var(--ui-small)] text-foreground outline-none placeholder:text-faint [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:text-[length:max(16px,var(--ui-small))]"
            :placeholder="searchPlaceholder ?? t('collectionsUi.filterMenu.search')"
            :aria-label="searchPlaceholder ?? t('collectionsUi.filterMenu.search')"
          />
        </div>
        <ListboxContent class="min-h-0 flex-1 overflow-y-auto p-1">
          <ListboxGroup
            v-for="group in groups"
            :key="group.name"
          >
            <ListboxGroupLabel
              v-if="group.name"
              class="px-2 pt-2 pb-1 text-[length:var(--ui-label)] font-semibold uppercase tracking-[0.02em] text-muted-foreground"
              >{{ group.name }}</ListboxGroupLabel
            >
            <ListboxItem
              v-for="option in group.options"
              :key="option.value"
              :value="option.value"
              :disabled="option.disabled"
              :class="itemClass"
            >
              <AppIcon
                v-if="option.icon"
                :name="option.icon"
                :size="14"
                class="shrink-0 text-muted-foreground"
              />
              <span class="min-w-0 flex-1 truncate">{{ option.label }}</span>
              <ListboxItemIndicator class="shrink-0 text-primary"
                ><AppIcon
                  name="check"
                  :size="14"
              /></ListboxItemIndicator>
            </ListboxItem>
          </ListboxGroup>
          <p
            v-if="!shown.length"
            class="m-0 px-2 py-3 text-center text-[length:var(--ui-small)] text-muted-foreground"
          >
            {{ t('collectionsUi.filterMenu.noMatches', { query }) }}
          </p>
        </ListboxContent>
        <div
          v-if="note || (multiple && clearLabel)"
          class="flex shrink-0 items-center gap-2 border-t [border-top-style:solid] border-border/60 px-2.5 py-1.5 text-[length:var(--ui-caption)] text-muted-foreground"
        >
          <span class="min-w-0 flex-1">{{ note }}</span>
          <UiButton
            v-if="multiple && clearLabel"
            variant="link"
            size="text"
            class="-mr-1.5 text-[length:var(--ui-caption)]"
            :disabled="!active"
            @click="model = []"
            >{{ clearLabel }}</UiButton
          >
        </div>
      </ListboxRoot>
    </UiPopoverContent>
  </UiPopover>
</template>
