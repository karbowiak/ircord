<script setup vapor lang="ts">
import type { ChannelRights } from '../../../shared/utils/irc'
import { modeInfo as channelModeInfo, parseChanmodes } from '../../../shared/utils/channel-modes'
import type { Conversation, Network } from '../../../shared/types/chat'

/** Channel settings → Modes: the flags and valued modes the network advertises in CHANMODES. */
const props = defineProps<{ c: Conversation; network?: Network; rights: ChannelRights }>()
const { t } = useI18n()
const translate = (key: string, values?: Record<string, string | number>) => t(key, values ?? {})
const { set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
// Used only by command handlers; avoid a reactive subscription outside a render effect.
const channelName = () => `#${props.c.name}`
const groups = computed(() => parseChanmodes(props.network?.isupport))
const modeInfo = computed(() => channelModeInfo(translate))
const info = (mode: string) =>
  modeInfo.value[mode] ?? {
    label: t('collectionsUi.channel.modes.serverSpecific'),
    description: t('collectionsUi.channel.modes.serverDescription'),
  }
/** Flags the network advertises, common ones first. */
const flags = computed(() =>
  [...groups.value.flags].sort(
    (a, b) => Number(!!modeInfo.value[b]) - Number(!!modeInfo.value[a]) || a.localeCompare(b),
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
  send(`MODE ${channelName()} ${set.value.has(mode) ? '-' : '+'}${mode}`)
/** Number inputs give numbers through v-model; compare everything as text. */
const valueOf = (mode: string) => String(values[mode] ?? '').trim()
function setParam(mode: string) {
  const value = valueOf(mode)
  if (value) send(`MODE ${channelName()} +${mode} ${value}`)
}
function clearParam(mode: string) {
  // Removing a key needs the key on some servers; send the current one when known.
  const current = groups.value.always.includes(mode) ? ` ${props.c.modeParams?.[mode] ?? '*'}` : ''
  send(`MODE ${channelName()} -${mode}${current}`)
}
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <section class="setting-group grid gap-3.5">
      <h3>{{ t('collectionsUi.channel.modes.accessAndBehaviour') }}</h3>
      <div class="setting-card">
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
              {{ info(mode).label }}</strong
            ><small
              >{{ info(mode).description
              }}<template v-if="modeInfo[mode]?.varies">{{
                ` ${t('collectionsUi.channel.modes.varies')}`
              }}</template></small
            ></span
          >
          <AppSwitch
            :model-value="set.has(mode)"
            :aria-label="
              t('collectionsUi.channel.modes.modeAccessLabel', { mode, label: info(mode).label })
            "
            :disabled="!rights.op"
            @update:model-value="toggle(mode)"
          />
        </div>
      </div>
    </section>
    <section
      v-if="paramModes.length"
      class="setting-group grid gap-3.5"
    >
      <h3>{{ t('collectionsUi.channel.modes.settingsWithValue') }}</h3>
      <div class="setting-card">
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
              {{ info(mode).label }}</strong
            ><small
              >{{ info(mode).description }}
              <template v-if="set.has(mode)">{{
                t('collectionsUi.channel.modes.currentlySet', {
                  value:
                    mode === 'k'
                      ? t('collectionsUi.channel.modes.set')
                      : (c.modeParams?.[mode] ?? ''),
                })
              }}</template></small
            ></span
          >
          <div class="param-controls flex gap-1.5 items-center">
            <UiInput
              v-model="values[mode]"
              class="w-40"
              :type="mode === 'l' ? 'number' : mode === 'k' ? 'password' : 'text'"
              :min="mode === 'l' ? 1 : undefined"
              :placeholder="
                mode === 'k'
                  ? set.has('k')
                    ? t('collectionsUi.channel.modes.newKey')
                    : t('collectionsUi.channel.modes.key')
                  : mode === 'l'
                    ? '50'
                    : t('collectionsUi.channel.modes.value')
              "
              :aria-label="t('collectionsUi.channel.modes.valueFor', { mode })"
              :disabled="!rights.op"
              autocomplete="off"
            /><UiButton
              variant="secondary"
              size="sm"
              :disabled="!rights.op || !valueOf(mode)"
            >
              {{ t('collectionsUi.channel.modes.setButton') }}</UiButton
            ><UiButton
              v-if="set.has(mode)"
              variant="secondary"
              size="sm"
              type="button"
              :disabled="!rights.op"
              @click="clearParam(mode)"
            >
              {{ t('collectionsUi.channel.modes.removeButton') }}
            </UiButton>
          </div>
        </form>
      </div>
    </section>
    <FieldHint>{{ t('collectionsUi.channel.modes.registrationHint') }}</FieldHint>
  </div>
</template>
