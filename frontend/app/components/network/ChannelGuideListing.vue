<script setup vapor lang="ts">
import type { components } from '../../../shared/generated/api'

type Listing = components['schemas']['ChannelGuideListing']

/**
 * Channel settings → Channel guide (#258): channel operators ask server owners to list the
 * channel in the guide new members pick from, and can withdraw the request. Owners approve,
 * edit or decline it in Administration.
 */
const props = defineProps<{ conversationId: string; name: string; listing: Listing }>()
const emit = defineEmits<{ update: [listing: Listing] }>()
const { t } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const { say } = useStatusAnnouncer()
const group = ref(props.listing.group ?? '')
const description = ref(props.listing.description ?? '')
const busy = ref(false)
const error = ref<unknown>()
const groupsId = `guide-groups-${props.conversationId}`

async function run(action: () => Promise<Listing>, done: string) {
  busy.value = true
  error.value = undefined
  try {
    const next = await action()
    emit('update', next)
    group.value = next.group ?? ''
    description.value = next.description ?? ''
    say(done)
  } catch (e) {
    error.value = e
  } finally {
    busy.value = false
  }
}
const request = () =>
  run(
    () =>
      apiClient().requestGuideListing(props.conversationId, {
        group: group.value.trim() || undefined,
        description: description.value.trim() || undefined,
      }),
    t('channelGuide.listing.sent'),
  )
const withdraw = () =>
  run(
    () => apiClient().withdrawGuideListing(props.conversationId),
    t('channelGuide.listing.withdrawn'),
  )
</script>
<template>
  <div class="channel-guide-listing grid gap-4 max-w-[560px]">
    <SettingsDescription>{{
      t('channelGuide.listing.intro', { channel: name })
    }}</SettingsDescription>
    <p
      class="guide-listing-status m-0 inline-flex w-fit items-center gap-2 rounded-full py-1.5 pr-3.5 pl-2.5 text-[length:var(--ui-caption)] font-[550] bg-secondary/45 text-muted-foreground"
      :data-status="listing.status"
    >
      <AppIcon
        :name="
          listing.status === 'listed' ? 'check' : listing.status === 'pending' ? 'clock' : 'compass'
        "
        :size="15"
      />{{ t(`channelGuide.listing.status.${listing.status}`) }}
    </p>
    <template v-if="listing.status === 'listed'">
      <p class="m-0 text-[13px] text-body">
        <span
          v-if="listing.group"
          class="mr-1.5 py-[1px] px-1.5 rounded-[4px] bg-selection text-primary text-[11px] font-[600]"
          >{{ listing.group }}</span
        >{{ listing.description || t('channelGuide.admin.noDescription') }}
      </p>
      <FieldHint>{{ t('channelGuide.listing.listedHelp') }}</FieldHint>
    </template>
    <form
      v-else-if="listing.operator"
      class="grid gap-3"
      @submit.prevent="request"
    >
      <AppInput
        :id="`guide-group-${conversationId}`"
        v-model="group"
        maxlength="40"
        :list="groupsId"
        :help="t('channelGuide.listing.groupHelp')"
        autocomplete="off"
      >
        <template #label
          >{{ t('channelGuide.listing.group') }} <small>{{ t('common.optional') }}</small></template
        >
      </AppInput>
      <datalist :id="groupsId">
        <option
          v-for="g in listing.groups"
          :key="g"
          :value="g"
        />
      </datalist>
      <AppInput
        :id="`guide-description-${conversationId}`"
        v-model="description"
        maxlength="120"
        :help="t('channelGuide.listing.descriptionHelp')"
        autocomplete="off"
      >
        <template #label
          >{{ t('channelGuide.listing.description') }}
          <small>{{ t('common.optional') }}</small></template
        >
      </AppInput>
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error"
      >
        {{ errorMessage(error) }}
      </UiAlert>
      <div class="flex flex-wrap items-center gap-2">
        <UiButton :disabled="busy">
          {{
            listing.status === 'pending'
              ? t('channelGuide.listing.update')
              : t('channelGuide.listing.request')
          }}
        </UiButton>
        <UiButton
          v-if="listing.status === 'pending'"
          type="button"
          variant="ghost"
          :disabled="busy"
          @click="withdraw"
          >{{ t('channelGuide.listing.withdraw') }}</UiButton
        >
      </div>
    </form>
    <FieldHint v-else>{{ t('channelGuide.listing.operatorsOnly') }}</FieldHint>
  </div>
</template>
