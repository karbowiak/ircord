<script setup vapor lang="ts">
import { useQuery } from '@pinia/colada'
import { relativeTime } from '../../../shared/utils/devices'
import {
  isAvatarFrame,
  languageName,
  localTime,
  statusShowing,
} from '../../../shared/utils/profile'

const props = defineProps<{ memberId: string }>()
const emit = defineEmits<{ close: []; edit: [] }>()
const chat = useChatStore()
const navigation = useConversationNavigation()
const { t, locale } = useI18n()
const $t = t
const member = computed(() => chat.member(props.memberId))
const nameStyle = useNameStyle()
/** Their chosen accent, else their name colour, tints the card so each one looks like its owner. */
const tint = computed(() =>
  member.value ? member.value.profileAccentColor || nameStyle(member.value).color : undefined,
)
const frame = computed(() => {
  const value = member.value?.avatarFrame
  return isAvatarFrame(value) && value !== 'none' ? value : undefined
})
const appearance = useAppearanceStore()
/** Their local time, refreshed each minute while the card is open. */
const now = ref(new Date())
const clock = setInterval(() => (now.value = new Date()), 60_000)
onBeforeUnmount(() => clearInterval(clock))
const theirTime = computed(() =>
  localTime(
    member.value?.timezone,
    now.value,
    locale.value,
    appearance.settings.timeFormat === '12h',
  ),
)
const status = computed(() =>
  member.value && statusShowing(member.value) ? member.value : undefined,
)
const homepage = computed(() => {
  const url = member.value?.homepageUrl
  if (!url) return undefined
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'https:'
      ? { href: parsed.href, label: parsed.host + parsed.pathname.replace(/\/$/, '') }
      : undefined
  } catch {
    return undefined
  }
})
/** Channels on this network you share with them (for yourself: the channels you are in). */
const channels = computed(() =>
  (chat.data?.conversations ?? [])
    .filter(
      (c) =>
        c.kind === 'channel' &&
        !c.party &&
        c.networkId === chat.networkId &&
        c.memberIds.includes(props.memberId),
    )
    .slice(0, 12),
)
/** Facts beyond the name; empty ones are left out. Further profile fields join this list. */
const details = computed(() => {
  const m = member.value
  if (!m) return []
  return [
    { label: t('profileDialog.network'), value: chat.network?.name },
    { label: t('profileDialog.account'), value: m.account },
    {
      label: t('profileDialog.realName'),
      // “IRCord” is the real name the backend fills in when none is set; it says nothing.
      value:
        m.realname && m.realname !== m.displayName && m.realname !== 'IRCord'
          ? m.realname
          : undefined,
    },
    { label: t('profileDialog.location'), value: m.location },
    { label: t('profileDialog.localTime'), value: theirTime.value },
    {
      label: t('profileDialog.languages'),
      value: m.languages?.length
        ? m.languages.map((tag) => languageName(tag, locale.value)).join(', ')
        : undefined,
    },
  ].filter((row) => !!row.value)
})
const sectionLabel = 'm-0 mb-1.5 text-[11px] font-[650] tracking-[0.05em] text-faint uppercase'
/** Label left, value right; long values wrap. */
const infoRows =
  'm-0 grid [&>div]:flex [&>div]:items-baseline [&>div]:justify-between [&>div]:gap-4 [&>div]:border-b [&>div]:[border-bottom-style:solid] [&>div]:border-border/50 [&>div]:py-1.5 [&>div:last-child]:border-b-0 [&_dt]:shrink-0 [&_dt]:text-[12px] [&_dt]:text-muted-foreground [&_dd]:m-0 [&_dd]:min-w-0 [&_dd]:text-right [&_dd]:text-[13px] [&_dd]:wrap-anywhere'
const ago = (iso: string) =>
  relativeTime(iso, Date.now(), locale.value, (key, values) =>
    t(`mediaUi.devices.${key}`, values ?? {}),
  )

/** WHOIS when the card opens: silent to the person, answered by the server. */
const whois = useQuery(() => ({
  key: ['whois', chat.networkId, member.value?.nick ?? ''],
  query: ({ signal }: { signal: AbortSignal }) =>
    apiClient().whois(chat.networkId, member.value!.nick, signal),
  enabled: !!chat.networkId && !!member.value?.nick,
  staleTime: 60_000,
}))
/** What the server says, as label and value rows; long values wrap under their label. */
const serverRows = computed(() => {
  const w = whois.data.value
  if (!w?.found) return []
  const rows: { label: string; value: string; code?: boolean }[] = []
  if (w.username || w.host)
    rows.push({ label: t('profileDialog.address'), value: `${w.username}@${w.host}`, code: true })
  if (w.server) rows.push({ label: t('profileDialog.server'), value: w.server })
  if (w.signedOnAt) rows.push({ label: t('profileDialog.connected'), value: ago(w.signedOnAt) })
  if (w.idleSeconds != null)
    rows.push({
      label: t('profileDialog.lastActive'),
      value: ago(new Date(Date.now() - w.idleSeconds * 1000).toISOString()),
    })
  rows.push({
    label: t('profileDialog.connection'),
    value: w.secure ? t('profileDialog.secure') : t('profileDialog.notSecure'),
  })
  return rows
})

/** CTCP goes to their client and may show there, so it is sent only on request. */
type CtcpCommand = 'VERSION' | 'TIME' | 'PING'
const ctcp = ref<Partial<Record<CtcpCommand, { answered: boolean; text: string }>>>({})
const asking = ref(false)
async function askClient() {
  const m = member.value
  if (!m || asking.value) return
  asking.value = true
  error.value = ''
  try {
    const commands: CtcpCommand[] = ['VERSION', 'TIME', 'PING']
    const replies = await Promise.all(
      commands.map((command) => apiClient().ctcp(chat.networkId, m.nick, command)),
    )
    ctcp.value = Object.fromEntries(
      replies.map((r) => [
        r.command,
        {
          answered: r.answered,
          text:
            r.command === 'PING'
              ? t('profileDialog.pingMs', { ms: r.latencyMs ?? 0 })
              : (r.reply ?? ''),
        },
      ]),
    )
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    asking.value = false
  }
}
const ctcpRows = computed(() =>
  (
    [
      ['VERSION', t('profileDialog.client')],
      ['TIME', t('profileDialog.theirTime')],
      ['PING', t('profileDialog.ping')],
    ] as const
  )
    .filter(([command]) => ctcp.value[command])
    .map(([command, label]) => ({
      label,
      value: ctcp.value[command]!.answered ? ctcp.value[command]!.text : t('profileDialog.noReply'),
      answered: ctcp.value[command]!.answered,
    })),
)

const error = ref('')
async function message() {
  try {
    await chat.direct.mutateAsync(props.memberId)
    emit('close')
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  }
}
function openChannel(id: string) {
  void navigation.open(id)
  emit('close')
}
/** Channel buttons stop their click, so the capture phase sees it first. */
function closeOnChannel(event: MouseEvent) {
  if ((event.target as HTMLElement | null)?.closest('.inline-channel')) emit('close')
}
</script>
<template>
  <AppDialog
    :title="member?.displayName || $t('profileDialog.title')"
    bare
    @close="emit('close')"
    ><div
      v-if="member"
      class="profile-card relative"
    >
      <!-- Their card background, faint behind everything below the banner. -->
      <img
        v-if="member.profileBackgroundUrl"
        :src="member.profileBackgroundUrl"
        alt=""
        aria-hidden="true"
        class="pointer-events-none absolute inset-0 size-full object-cover opacity-20"
      />
      <!-- The banner: their own image, else their picture blurred and washed in their colour. -->
      <div
        class="profile-banner relative h-32 overflow-hidden"
        :class="`tone-${member.color}`"
      >
        <img
          v-if="member.bannerUrl"
          :src="member.bannerUrl"
          alt=""
          aria-hidden="true"
          class="absolute inset-0 size-full object-cover"
        />
        <template v-else>
          <img
            v-if="member.avatarUrl"
            :src="member.avatarUrl"
            alt=""
            aria-hidden="true"
            class="absolute inset-0 size-full scale-150 object-cover opacity-80 blur-2xl saturate-150"
          />
          <div
            class="absolute inset-0 opacity-45 mix-blend-color"
            :style="{ background: tint }"
          />
        </template>
        <div
          class="absolute inset-0 [background:linear-gradient(180deg,transparent_30%,var(--main))]"
        />
      </div>
      <div class="profile-card-body relative -mt-14 px-5 pb-5">
        <div class="flex items-end gap-3.5">
          <div
            class="flex shrink-0 rounded-full shadow-[0_0_0_5px_var(--main),0_8px_24px_rgb(0_0_0/35%)] [&_.presence]:border-[var(--main)]"
            :class="frame ? ['avatar-frame', `avatar-frame-${frame}`] : undefined"
            :style="{ '--frame': tint }"
          >
            <UserAvatar
              :member="member"
              size="large"
              presence
            />
          </div>
          <div class="flex min-w-0 flex-wrap items-center gap-1.5 pb-1">
            <span
              v-if="member.role === 'operator'"
              class="inline-flex items-center gap-1 rounded-full bg-success/15 px-2 py-0.5 text-[11px] font-[650] text-success"
              ><AppIcon
                name="shield"
                :size="12"
              />{{ $t('profileDialog.operator') }}</span
            ><span
              v-if="whois.data.value?.operator"
              class="inline-flex items-center gap-1 rounded-full bg-warning/15 px-2 py-0.5 text-[11px] font-[650] text-warning"
              ><AppIcon
                name="shield"
                :size="12"
              />{{ $t('profileDialog.ircOperator') }}</span
            ><span
              v-if="member.role === 'bot'"
              class="bot-badge inline-flex items-center rounded-full bg-selection px-2 py-0.5 text-[11px] font-[650] tracking-[0.04em] text-primary"
              >{{ $t('mediaUi.profile.bot') }}</span
            ><span
              v-if="member.device"
              class="inline-flex items-center gap-1 rounded-full bg-secondary/60 px-2 py-0.5 text-[11px] text-muted-foreground"
              ><AppIcon
                :name="member.device === 'tablet' ? 'tablet' : 'smartphone'"
                :size="12"
              />{{ $t(`profileDialog.device.${member.device}`) }}</span
            >
          </div>
        </div>
        <h3
          class="mt-3 text-[24px] leading-tight font-[750] tracking-[-0.02em] wrap-anywhere"
          :style="nameStyle(member)"
        >
          {{ member.displayName }}
        </h3>
        <p
          v-if="member.headline"
          class="profile-headline mt-0.5 text-[14px] text-body wrap-anywhere"
        >
          {{ member.headline }}
        </p>
        <p
          class="profile-nick mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[13px] text-muted-foreground"
        >
          @{{ member.nick
          }}<span
            v-if="member.pronouns"
            class="rounded-full bg-secondary/60 px-2 py-px text-[12px]"
            >{{ member.pronouns }}</span
          ><span class="flex items-center gap-[5px] capitalize"
            ><i
              class="status-dot inline-block h-[7px] w-[7px] shrink-0 rounded-full bg-success"
              :class="{
                'is-offline': member.status === 'offline',
                'is-away': member.status === 'away',
              }"
            />{{ $t(`profileDialog.status.${member.status}`) }}</span
          ><span
            v-if="member.status === 'away' && member.awayMessage"
            class="profile-away min-w-0 truncate italic"
            >“{{ member.awayMessage }}”</span
          >
        </p>
        <p
          v-if="status"
          class="profile-status mt-3 inline-flex max-w-full items-center gap-2 rounded-full bg-secondary/45 py-1 pr-3.5 pl-2.5 text-[13px] text-body shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
        >
          <StatusEmoji
            v-if="status.statusEmoji"
            :value="status.statusEmoji"
            class="text-[15px] leading-none"
          /><span
            v-if="status.statusText"
            class="min-w-0 truncate"
            >{{ status.statusText }}</span
          >
        </p>
        <!-- A #channel in the bio opens the channel behind the dialog, so the dialog steps aside. -->
        <p
          v-if="member.bio"
          class="profile-bio mt-4 border-l-2 [border-left-style:solid] pl-3 text-[14px] leading-[1.55] whitespace-pre-wrap text-body wrap-anywhere"
          :style="{ borderColor: tint }"
          @click.capture="closeOnChannel"
        >
          <MessageRuns :text="member.bio" />
        </p>
        <a
          v-if="homepage"
          :href="homepage.href"
          target="_blank"
          rel="noopener noreferrer nofollow"
          class="mt-3 inline-flex max-w-full items-center gap-1.5 text-[13px] text-primary hover:underline"
          ><AppIcon
            name="link"
            :size="13"
          /><span class="truncate">{{ homepage.label }}</span></a
        >
        <section
          v-if="member.interests?.length"
          class="mt-4"
          :aria-label="$t('profileDialog.interests')"
        >
          <h4 :class="sectionLabel">{{ $t('profileDialog.interests') }}</h4>
          <div class="flex flex-wrap gap-1.5">
            <span
              v-for="interest in member.interests"
              :key="interest"
              class="rounded-full px-2.5 py-1 text-[12px] font-[550] text-body"
              :style="{
                background: `color-mix(in srgb, ${tint ?? 'var(--accent)'} 16%, var(--raised))`,
              }"
              >{{ interest }}</span
            >
          </div>
        </section>
        <section
          v-if="channels.length"
          class="mt-4"
          :aria-label="$t('profileDialog.channels')"
        >
          <h4 class="m-0 mb-1.5 text-[11px] font-[650] tracking-[0.05em] text-faint uppercase">
            {{
              member.id === 'you'
                ? $t('profileDialog.yourChannels')
                : $t('profileDialog.sharedChannels')
            }}
          </h4>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="c in channels"
              :key="c.id"
              type="button"
              class="inline-flex items-center gap-1 rounded-full bg-secondary/45 py-1 pr-2.5 pl-2 text-[12px] font-[550] text-body shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] transition-colors hover:bg-accent hover:text-foreground"
              @click="openChannel(c.id)"
            >
              <AppIcon
                name="hash"
                :size="12"
                class="text-faint"
              />{{ c.name.replace(/^#/, '') }}
            </button>
          </div>
        </section>
        <dl
          v-if="details.length"
          class="stat-strip mt-4"
        >
          <div
            v-for="row in details"
            :key="row.label"
          >
            <dt>{{ row.label }}</dt>
            <dd>{{ row.value }}</dd>
          </div>
        </dl>
        <!-- WHOIS, asked when the card opens. -->
        <section
          v-if="whois.isLoading.value || serverRows.length"
          class="mt-4"
          :aria-label="$t('profileDialog.fromServer')"
        >
          <h4 :class="sectionLabel">{{ $t('profileDialog.fromServer') }}</h4>
          <p
            v-if="whois.isLoading.value && !serverRows.length"
            class="m-0 text-[12px] text-faint"
            role="status"
          >
            {{ $t('profileDialog.lookingUp') }}
          </p>
          <dl
            v-else
            :class="infoRows"
          >
            <div
              v-for="row in serverRows"
              :key="row.label"
            >
              <dt>{{ row.label }}</dt>
              <dd :class="{ 'font-code text-[12px]': row.code }">{{ row.value }}</dd>
            </div>
          </dl>
        </section>
        <!-- CTCP reaches their client, which may show that you asked, so it waits for a click. -->
        <section
          v-if="member.id !== 'you'"
          class="mt-4"
          :aria-label="$t('profileDialog.fromClient')"
        >
          <div class="flex items-center justify-between gap-3">
            <h4 :class="[sectionLabel, 'mb-0']">{{ $t('profileDialog.fromClient') }}</h4>
            <button
              type="button"
              class="inline-flex items-center gap-1.5 rounded-full bg-secondary/45 px-3 py-1 text-[12px] font-[550] text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] transition-colors hover:bg-accent hover:text-foreground disabled:opacity-60"
              :disabled="asking"
              @click="askClient"
            >
              <AppIcon
                name="refresh"
                :size="12"
                :class="{ 'animate-spin': asking }"
              />{{
                asking
                  ? $t('profileDialog.asking')
                  : ctcpRows.length
                    ? $t('profileDialog.askAgain')
                    : $t('profileDialog.askClient')
              }}
            </button>
          </div>
          <p
            v-if="!ctcpRows.length"
            class="m-0 mt-1.5 text-[12px] text-faint"
          >
            {{ $t('profileDialog.askClientHelp') }}
          </p>
          <dl
            v-else
            :class="[infoRows, 'mt-1.5']"
          >
            <div
              v-for="row in ctcpRows"
              :key="row.label"
            >
              <dt>{{ row.label }}</dt>
              <dd :class="{ 'text-faint italic': !row.answered }">{{ row.value }}</dd>
            </div>
          </dl>
        </section>
        <UiAlert
          v-if="error"
          as="p"
          variant="destructive"
          presentation="inline"
          class="form-error mt-3"
        >
          {{ error }}
        </UiAlert>
        <UiButton
          v-if="member.id === 'you'"
          class="mt-5 w-full rounded-full"
          @click="emit('edit')"
        >
          <AppIcon
            name="settings"
            :size="17"
          />{{ $t('profileDialog.edit') }}</UiButton
        ><UiButton
          v-else
          class="mt-5 w-full rounded-full"
          :disabled="chat.direct.isLoading"
          @click="message"
        >
          <AppIcon
            name="message"
            :size="17"
          />{{ chat.direct.isLoading ? $t('profileDialog.opening') : $t('profileDialog.message') }}
        </UiButton>
      </div>
    </div></AppDialog
  >
</template>
