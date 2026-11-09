<script setup lang="ts">
import type { ChannelRights } from '../../shared/utils/irc'
import { modeInfo, parseChanmodes } from '../../shared/utils/channel-modes'
import type { Conversation, Network } from '../../shared/types/chat'

/** Channel settings → Modes: the flags and valued modes the network advertises in CHANMODES. */
const props = defineProps<{ c: Conversation; network?: Network; rights: ChannelRights }>()
const { name, set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
const groups = computed(() => parseChanmodes(props.network?.isupport))
/** Flags the network advertises, common ones first. */
const flags = computed(() =>
  [...groups.value.flags].sort(
    (a, b) => Number(!!modeInfo[b]) - Number(!!modeInfo[a]) || a.localeCompare(b),
  ),
)
const paramModes = computed(() => [...groups.value.always, ...groups.value.whenSet])
const values = reactive<Record<string, string>>({})
watch(
  () => props.c.modeParams,
  (params) => {
    for (const mode of paramModes.value) values[mode] = mode === 'k' ? '' : (params?.[mode] ?? '')
  },
  { immediate: true },
)
const toggle = (mode: string) =>
  send(`MODE ${name.value} ${set.value.has(mode) ? '-' : '+'}${mode}`)
/** Number inputs give numbers through v-model; compare everything as text. */
const valueOf = (mode: string) => String(values[mode] ?? '').trim()
function setParam(mode: string) {
  const value = valueOf(mode)
  if (value) send(`MODE ${name.value} +${mode} ${value}`)
}
function clearParam(mode: string) {
  // Removing a key needs the key on some servers; send the current one when known.
  const current = groups.value.always.includes(mode) ? ` ${props.c.modeParams?.[mode] ?? '*'}` : ''
  send(`MODE ${name.value} -${mode}${current}`)
}
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <section class="setting-group grid gap-3.5">
      <h3>Access and behaviour</h3>
      <div
        v-for="mode in flags"
        :key="mode"
        class="setting-row flex items-center justify-between gap-4"
      >
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong
            ><code
              class="mode-letter font-code text-[12px] font-[600] py-[1px] px-[5px] mr-1 rounded-[4px] bg-rail text-primary"
              >+{{ mode }}</code
            >
            {{ modeInfo[mode]?.label ?? 'Server-specific mode' }}</strong
          ><small
            >{{ modeInfo[mode]?.description ?? 'This network defines what this mode does.'
            }}<template v-if="modeInfo[mode]?.varies">
              Its exact meaning depends on the server.</template
            ></small
          ></span
        >
        <AppSwitch
          :model-value="set.has(mode)"
          :aria-label="`+${mode} ${modeInfo[mode]?.label ?? ''}`"
          :disabled="!rights.op"
          @update:model-value="toggle(mode)"
        />
      </div>
    </section>
    <section
      v-if="paramModes.length"
      class="setting-group grid gap-3.5"
    >
      <h3>Settings with a value</h3>
      <form
        v-for="mode in paramModes"
        :key="mode"
        class="setting-row param-mode flex items-center justify-between gap-4 flex-wrap"
        @submit.prevent="setParam(mode)"
      >
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong
            ><code
              class="mode-letter font-code text-[12px] font-[600] py-[1px] px-[5px] mr-1 rounded-[4px] bg-rail text-primary"
              >+{{ mode }}</code
            >
            {{ modeInfo[mode]?.label ?? 'Server-specific mode' }}</strong
          ><small
            >{{ modeInfo[mode]?.description ?? 'This network defines what this mode does.' }}
            <template v-if="set.has(mode)"
              >Currently <strong>{{ mode === 'k' ? 'set' : c.modeParams?.[mode] }}</strong
              >.</template
            ></small
          ></span
        >
        <div class="param-controls flex gap-1.5 items-center">
          <UiInput
            v-model="values[mode]"
            class="w-40"
            :type="mode === 'l' ? 'number' : mode === 'k' ? 'password' : 'text'"
            :min="mode === 'l' ? 1 : undefined"
            :placeholder="
              mode === 'k' ? (set.has('k') ? 'New key' : 'Key') : mode === 'l' ? '50' : 'Value'
            "
            :aria-label="`Value for +${mode}`"
            :disabled="!rights.op"
            autocomplete="off"
          /><UiButton
            variant="secondary"
            size="sm"
            :disabled="!rights.op || !valueOf(mode)"
          >
            Set</UiButton
          ><UiButton
            v-if="set.has(mode)"
            variant="secondary"
            size="sm"
            type="button"
            :disabled="!rights.op"
            @click="clearParam(mode)"
          >
            Remove
          </UiButton>
        </div>
      </form>
    </section>
    <FieldHint> Registration, founders and access lists are on the Registration tab. </FieldHint>
  </div>
</template>
