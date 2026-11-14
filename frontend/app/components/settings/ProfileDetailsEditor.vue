<script setup vapor lang="ts">
import type { Member } from '../../../shared/types/chat'
import {
  avatarFrames,
  isAvatarFrame,
  languageName,
  statusDeadline,
} from '../../../shared/utils/profile'
import { relativeTime } from '../../../shared/utils/devices'
import { fieldChanges, mergeFields } from './profile-draft'

/**
 * Settings → Profile, below name, picture and bio: the richer profile fields this server shares
 * with other people (status, headline, pronouns, where and when they are, what they like, and
 * how the card looks). Saved together with one button, like the fields above.
 */
const props = defineProps<{ networkId: string }>()
const chat = useChatStore()
const { t, locale } = useI18n()
const $t = t
const user = computed(() => chat.selfFor(props.networkId))

type Details = {
  statusEmoji: string
  statusText: string
  headline: string
  pronouns: string
  location: string
  timezone: string
  homepageUrl: string
  languages: string[]
  interests: string[]
  bannerUrl: string
  profileBackgroundUrl: string
  profileAccentColor: string
  avatarFrame: string
}
const fields = (m: Member | undefined): Details => ({
  statusEmoji: m?.statusEmoji ?? '',
  statusText: m?.statusText ?? '',
  headline: m?.headline ?? '',
  pronouns: m?.pronouns ?? '',
  location: m?.location ?? '',
  timezone: m?.timezone ?? '',
  homepageUrl: m?.homepageUrl ?? '',
  languages: m?.languages ?? [],
  interests: m?.interests ?? [],
  bannerUrl: m?.bannerUrl ?? '',
  profileBackgroundUrl: m?.profileBackgroundUrl ?? '',
  profileAccentColor: m?.profileAccentColor ?? '',
  avatarFrame: m?.avatarFrame ?? '',
})
const baseline = ref<Details>(fields(user.value))
const draft = reactive<Details>(fields(user.value))
/** Other devices' changes refresh untouched fields; fields edited here keep their text. */
watch(user, (member) => {
  const latest = fields(member)
  Object.assign(draft, mergeFields(baseline.value, { ...draft }, latest))
  baseline.value = latest
})
const changes = computed(() => fieldChanges(baseline.value, { ...draft }))
const dirty = computed(() => Object.keys(changes.value).length > 0 || clearAfter.value !== 'keep')

/** When the status clears: chosen when it is set; “keep” leaves the current deadline alone. */
const clearAfter = ref<'keep' | 'never' | '30m' | '1h' | '4h' | 'today' | 'week'>('keep')
const clearOptions = computed(() =>
  (['never', '30m', '1h', '4h', 'today', 'week'] as const).map((value) => ({
    value,
    label: t(`profileDetails.clear.${value}`),
  })),
)
const statusClears = computed(() =>
  user.value?.statusExpiresAt
    ? relativeTime(user.value.statusExpiresAt, Date.now(), locale.value, (key, values) =>
        t(`mediaUi.devices.${key}`, values ?? {}),
      )
    : '',
)

const timezones = (() => {
  try {
    return Intl.supportedValuesOf('timeZone')
  } catch {
    return ['UTC']
  }
})()
function useDeviceTimezone() {
  draft.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
}

function addLanguage(value: string) {
  try {
    const [tag] = Intl.getCanonicalLocales(value.trim())
    if (!tag || draft.languages.includes(tag) || draft.languages.length >= 10) return false
    draft.languages = [...draft.languages, tag]
  } catch {
    return false
  }
}
function addInterest(value: string) {
  const v = value.trim()
  if (!v || new TextEncoder().encode(v).length > 32 || draft.interests.length >= 10) return false
  if (draft.interests.some((i) => i.toLowerCase() === v.toLowerCase())) return false
  draft.interests = [...draft.interests, v]
}

const imageFields = ['bannerUrl', 'profileBackgroundUrl'] as const
/** Banner and background are uploaded like attachments, then saved by their address. */
const uploading = ref<'' | 'bannerUrl' | 'profileBackgroundUrl'>('')
async function upload(field: 'bannerUrl' | 'profileBackgroundUrl', event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  error.value = ''
  uploading.value = field
  try {
    const uploaded = await apiClient().upload(file, props.networkId)
    draft[field] = uploaded.url
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    uploading.value = ''
  }
}

const error = ref('')
const saving = ref(false)
const saved = ref(false)
watch(draft, () => (saved.value = false), { deep: true })
async function save() {
  const input: Record<string, unknown> = { ...changes.value }
  const statusChanged = 'statusText' in input || 'statusEmoji' in input
  if (clearAfter.value !== 'keep' || statusChanged)
    input.statusExpiresAt =
      (clearAfter.value === 'keep' ? null : statusDeadline(clearAfter.value)) ?? ''
  error.value = ''
  saving.value = true
  try {
    const member = await chat.networkProfile.mutateAsync({ id: props.networkId, input })
    baseline.value = fields(member)
    Object.assign(draft, fields(member))
    clearAfter.value = 'keep'
    saved.value = true
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    saving.value = false
  }
}

const groupTitle = 'm-0 text-[12px] font-[650] tracking-[0.05em] text-faint uppercase'
const card =
  'grid gap-3 rounded-[var(--radius-large)] bg-card p-4 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]'
</script>

<template>
  <form
    class="profile-details mt-6 grid gap-4"
    @submit.prevent="save"
  >
    <SettingsSubtitle>{{ $t('profileDetails.title') }}</SettingsSubtitle>
    <FieldHint class="m-0">{{ $t('profileDetails.help') }}</FieldHint>

    <section
      :class="card"
      data-setting="profile-status"
    >
      <h4 :class="groupTitle">{{ $t('profileDetails.status') }}</h4>
      <div
        class="grid gap-2 [grid-template-columns:calc(4.5rem*var(--interface-scale,1))_minmax(0,1fr)]"
      >
        <UiInput
          v-model="draft.statusEmoji"
          class="text-center"
          maxlength="16"
          :placeholder="$t('profileDetails.statusEmojiPlaceholder')"
          :aria-label="$t('profileDetails.statusEmoji')"
        />
        <UiInput
          v-model="draft.statusText"
          maxlength="100"
          :placeholder="$t('profileDetails.statusPlaceholder')"
          :aria-label="$t('profileDetails.statusText')"
        />
      </div>
      <label class="flex flex-wrap items-center gap-2 text-[13px] text-muted-foreground">
        {{ $t('profileDetails.clearAfter') }}
        <select
          v-model="clearAfter"
          class="rounded-md border border-border bg-background px-2 py-1 text-foreground"
        >
          <option value="keep">
            {{
              statusClears
                ? $t('profileDetails.keepDeadline', { when: statusClears })
                : $t('profileDetails.clear.never')
            }}
          </option>
          <option
            v-for="option in clearOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
      </label>
    </section>

    <section
      :class="card"
      data-setting="profile-about"
    >
      <h4 :class="groupTitle">{{ $t('profileDetails.about') }}</h4>
      <div
        class="grid gap-3 [grid-template-columns:repeat(auto-fit,minmax(200px,1fr))] [&>*]:min-w-0"
      >
        <AppField
          id="profile-headline"
          :label="$t('profileDetails.headline')"
          class="col-span-full"
        >
          <template #default="control">
            <UiInput
              v-bind="control"
              v-model="draft.headline"
              maxlength="120"
              :placeholder="$t('profileDetails.headlinePlaceholder')"
            />
          </template>
        </AppField>
        <AppField
          id="profile-pronouns"
          :label="$t('profileDetails.pronouns')"
        >
          <template #default="control">
            <UiInput
              v-bind="control"
              v-model="draft.pronouns"
              maxlength="40"
              :placeholder="$t('profileDetails.pronounsPlaceholder')"
            />
          </template>
        </AppField>
        <AppField
          id="profile-location"
          :label="$t('profileDetails.location')"
        >
          <template #default="control">
            <UiInput
              v-bind="control"
              v-model="draft.location"
              maxlength="120"
              :placeholder="$t('profileDetails.locationPlaceholder')"
            />
          </template>
        </AppField>
        <AppField
          id="profile-timezone"
          :label="$t('profileDetails.timezone')"
        >
          <template #default="control">
            <div class="flex min-w-0 gap-2">
              <select
                :id="control.id"
                v-model="draft.timezone"
                class="min-w-0 flex-1 rounded-md border border-border bg-background px-3 py-2 text-foreground"
              >
                <option value="">{{ $t('profileDetails.timezoneNone') }}</option>
                <option
                  v-for="zone in timezones"
                  :key="zone"
                  :value="zone"
                >
                  {{ zone }}
                </option>
              </select>
              <UiButton
                variant="secondary"
                size="sm"
                type="button"
                class="self-center"
                @click="useDeviceTimezone"
              >
                {{ $t('profileDetails.useMine') }}
              </UiButton>
            </div>
          </template>
        </AppField>
        <AppField
          id="profile-homepage"
          :label="$t('profileDetails.homepage')"
        >
          <template #default="control">
            <UiInput
              v-bind="control"
              v-model="draft.homepageUrl"
              type="url"
              inputmode="url"
              maxlength="300"
              :placeholder="$t('profileDetails.homepagePlaceholder')"
            />
          </template>
        </AppField>
      </div>
    </section>

    <section
      :class="card"
      data-setting="profile-interests"
    >
      <h4 :class="groupTitle">{{ $t('profileDetails.languagesAndInterests') }}</h4>
      <div class="grid gap-1.5">
        <span class="text-[13px] font-[550]">{{ $t('profileDetails.languages') }}</span>
        <ChipInput
          :items="draft.languages"
          :add="addLanguage"
          :remove="(value) => (draft.languages = draft.languages.filter((l) => l !== value))"
          :remove-label="
            (value) => $t('profileDetails.removeItem', { item: languageName(value, locale) })
          "
          :aria-label="$t('profileDetails.languages')"
          :placeholder="$t('profileDetails.languagesPlaceholder')"
          maxlength="32"
        />
        <FieldHint
          v-if="draft.languages.length"
          class="m-0"
          >{{ draft.languages.map((l) => languageName(l, locale)).join(', ') }}</FieldHint
        >
      </div>
      <div class="grid gap-1.5">
        <span class="text-[13px] font-[550]">{{ $t('profileDetails.interests') }}</span>
        <ChipInput
          :items="draft.interests"
          :add="addInterest"
          :remove="(value) => (draft.interests = draft.interests.filter((i) => i !== value))"
          :remove-label="(value) => $t('profileDetails.removeItem', { item: value })"
          :aria-label="$t('profileDetails.interests')"
          :placeholder="$t('profileDetails.interestsPlaceholder')"
          maxlength="32"
        />
      </div>
    </section>

    <section
      :class="card"
      data-setting="profile-look"
    >
      <h4 :class="groupTitle">{{ $t('profileDetails.look') }}</h4>
      <div
        class="grid gap-3 [grid-template-columns:repeat(auto-fit,minmax(200px,1fr))] [&>*]:min-w-0"
      >
        <div
          v-for="field in imageFields"
          :key="field"
          class="grid gap-1.5"
        >
          <span class="text-[13px] font-[550]">{{
            field === 'bannerUrl' ? $t('profileDetails.banner') : $t('profileDetails.background')
          }}</span>
          <div
            class="relative grid aspect-[3/1] place-items-center overflow-hidden rounded-md bg-secondary text-[12px] text-faint"
          >
            <img
              v-if="draft[field]"
              :src="draft[field]"
              alt=""
              class="absolute inset-0 size-full object-cover"
            />
            <span v-else>{{
              field === 'bannerUrl'
                ? $t('profileDetails.noBanner')
                : $t('profileDetails.noBackground')
            }}</span>
          </div>
          <div class="flex gap-2">
            <label
              class="inline-flex cursor-pointer items-center gap-1.5 rounded-full bg-secondary/45 px-3 py-1 text-[12px] font-[550] text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent hover:text-foreground"
              ><AppIcon
                name="image"
                :size="13" />{{
                uploading === field
                  ? $t('profileDetails.uploading')
                  : $t('profileDetails.chooseImage')
              }}<input
                type="file"
                class="sr-only"
                accept="image/png,image/jpeg,image/webp,image/gif"
                :disabled="!!uploading"
                @change="upload(field, $event)"
            /></label>
            <UiButton
              v-if="draft[field]"
              variant="ghost"
              size="sm"
              type="button"
              @click="draft[field] = ''"
            >
              {{ $t('profileDetails.removeImage') }}
            </UiButton>
          </div>
        </div>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <span class="grid gap-0.5"
          ><span class="text-[13px] font-[550]">{{ $t('profileDetails.accent') }}</span
          ><small class="text-[12px] text-muted-foreground">{{
            $t('profileDetails.accentHelp')
          }}</small></span
        >
        <span class="flex items-center gap-2">
          <input
            :value="draft.profileAccentColor || '#a78bfa'"
            class="h-[26px] w-9 rounded-sm border border-solid border-border bg-transparent p-0"
            type="color"
            :aria-label="$t('profileDetails.accent')"
            @input="draft.profileAccentColor = ($event.target as HTMLInputElement).value"
          />
          <UiButton
            v-if="draft.profileAccentColor"
            variant="ghost"
            size="sm"
            type="button"
            @click="draft.profileAccentColor = ''"
          >
            {{ $t('profileDetails.automatic') }}
          </UiButton>
        </span>
      </div>
      <div class="grid gap-1.5">
        <span class="text-[13px] font-[550]">{{ $t('profileDetails.frame') }}</span>
        <div
          class="flex flex-wrap gap-3"
          role="radiogroup"
          :aria-label="$t('profileDetails.frame')"
        >
          <button
            v-for="frame in avatarFrames"
            :key="frame"
            type="button"
            role="radio"
            :aria-checked="(draft.avatarFrame || 'none') === frame"
            class="grid justify-items-center gap-1.5 rounded-md p-2 text-[11px] text-muted-foreground hover:bg-accent aria-checked:bg-selection aria-checked:text-foreground"
            @click="draft.avatarFrame = frame === 'none' ? '' : frame"
          >
            <span
              class="m-1.5 flex"
              :class="
                isAvatarFrame(frame) && frame !== 'none'
                  ? ['avatar-frame', `avatar-frame-${frame}`]
                  : ''
              "
              :style="
                draft.profileAccentColor ? { '--frame': draft.profileAccentColor } : undefined
              "
              ><UserAvatar
                :member="user"
                size="normal"
            /></span>
            {{ $t(`profileDetails.frames.${frame}`) }}
          </button>
        </div>
      </div>
    </section>

    <div class="flex items-center justify-end gap-2">
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mr-auto"
      >
        {{ error }}
      </UiAlert>
      <span
        v-else-if="saved"
        class="mr-auto flex items-center gap-[5px] text-[13px] text-success"
        role="status"
        ><AppIcon
          name="check"
          :size="15"
        />{{ $t('profileDetails.saved') }}</span
      ><UiButton :disabled="!dirty || saving || !!uploading">
        {{ saving ? $t('common.saving') : $t('common.saveChanges') }}
      </UiButton>
    </div>
  </form>
</template>
