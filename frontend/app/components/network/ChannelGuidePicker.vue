<script setup vapor lang="ts">
import {
  guideKey,
  guideSections,
  type GuideChannel,
  type GuideGroup,
} from '../../../shared/utils/channel-guide'

/**
 * The channel guide as a tickable list (#258), shared by the welcome dialog, Join existing and
 * the owners' preview. Channels for everyone are ticked and locked; channels you're in show as
 * joined. The parent owns the selection and the join request.
 */
const props = defineProps<{
  groups: GuideGroup[]
  channels: GuideChannel[]
  joined: Set<string>
  disabled?: boolean
}>()
const selected = defineModel<string[]>({ default: () => [] })
const { t, n } = useI18n()
const $t = t
const sections = computed(() => guideSections(props.groups, props.channels))
const heading = (kind: string, name?: string) =>
  kind === 'everyone'
    ? t('channelGuide.forEveryone')
    : kind === 'more'
      ? t('channelGuide.moreChannels')
      : (name ?? '')
const locked = (c: GuideChannel) => c.state === 'everyone' || props.joined.has(guideKey(c.channel))
const checked = (c: GuideChannel) => locked(c) || selected.value.includes(guideKey(c.channel))
function toggle(c: GuideChannel, value: boolean) {
  const key = guideKey(c.channel)
  const rest = selected.value.filter((k) => k !== key)
  selected.value = value ? [...rest, key] : rest
}
const status = (c: GuideChannel) =>
  props.joined.has(guideKey(c.channel))
    ? t('join.joined')
    : c.state === 'everyone'
      ? t('channelGuide.everyoneJoins')
      : ''
</script>
<template>
  <div class="channel-guide grid gap-4">
    <section
      v-for="section in sections"
      :key="section.kind + (section.name ?? '')"
      class="grid gap-1"
      :aria-label="heading(section.kind, section.name)"
    >
      <h3
        class="m-0 px-2 text-[11px] font-[650] tracking-[0.05em] uppercase text-faint"
        aria-hidden="true"
      >
        {{ heading(section.kind, section.name) }}
      </h3>
      <label
        v-for="c in section.channels"
        :key="c.channel"
        class="channel-guide-row flex items-center gap-2.5 rounded-sm p-2 text-muted-foreground"
        :class="locked(c) || disabled ? 'cursor-default' : 'cursor-pointer hover:bg-accent'"
        :data-channel="c.channel"
      >
        <UiCheckbox
          :model-value="checked(c)"
          :disabled="locked(c) || disabled"
          :aria-describedby="`guide-${guideKey(c.channel).slice(1)}-about`"
          @update:model-value="toggle(c, !!$event)"
        /><span class="flex min-w-0 flex-1 flex-col"
          ><span class="flex min-w-0 items-center gap-1.5"
            ><strong class="truncate text-[14px] text-foreground">{{ c.channel }}</strong
            ><span
              v-if="c.nsfw"
              class="nsfw-badge text-[10px] font-[650] py-[1px] px-1.5 rounded-[4px] bg-destructive/12 text-destructive"
              >{{ $t('channelGuide.nsfw') }}</span
            ></span
          ><small
            :id="`guide-${guideKey(c.channel).slice(1)}-about`"
            class="overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-faint"
            ><template v-if="c.description">{{ c.description }}</template
            ><MessageContent
              v-else
              :text="c.topic || $t('join.noTopic')" /></small></span
        ><span
          v-if="status(c)"
          class="joined-badge inline-flex items-center gap-1 text-[11px] font-[600] py-[1px] px-1.5 rounded-[4px] bg-selection text-primary"
          ><AppIcon
            v-if="c.state === 'everyone' && !joined.has(guideKey(c.channel))"
            name="lock"
            :size="11"
          />{{ status(c) }}</span
        ><span
          v-if="c.members !== undefined"
          class="channel-member-count flex items-center gap-1 text-[12px] text-faint min-w-16 justify-end"
          ><AppIcon
            name="users"
            :size="13"
          /><span aria-hidden="true">{{ n(c.members) }}</span
          ><span class="sr-only">{{
            $t('join.userCount', { count: n(c.members) }, c.members)
          }}</span></span
        >
      </label>
    </section>
  </div>
</template>
