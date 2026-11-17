<script setup lang="ts">
import { collaborators, latest } from '../data'
import { site } from '../site'
import type shots from '../shots.json'

const description =
  'IRCord is a web client and a bouncer in one. It keeps your IRC networks online and saves the messages you miss.'
useSeoMeta({
  title: 'IRCord: IRC that stays connected',
  description,
  ogTitle: 'IRCord: IRC that stays connected',
  ogDescription: description,
  ogUrl: `${site.url}/`,
})
useHead({ link: [{ rel: 'canonical', href: `${site.url}/` }] })
const names = new Intl.ListFormat('en', { type: 'conjunction' }).format(collaborators)

const features: {
  title: string
  text: string
  shot: keyof typeof shots
  alt: string
  /** How the picture fills the card; by default it is cropped from the top left. */
  fit?: string
}[] = [
  {
    title: 'Message history and search',
    text: 'Read what was said while you were away. Search your saved messages by word, person, channel or date.',
    shot: 'search',
    alt: 'The search page with two messages that contain the word desk.',
  },
  {
    title: 'Files, pictures and link previews',
    text: 'Upload files and see pictures, video and link previews in the conversation.',
    shot: 'media',
    alt: 'A message with a picture of a road through red mountains, and a reply below it.',
  },
  {
    title: 'Profiles and status',
    text: 'Set a picture, a status and a short profile. Other people see them on your profile card.',
    shot: 'profile',
    alt: 'The profile card of Mika, an operator, with the channels you share.',
    fit: 'object-cover object-[50%_35%]',
  },
  {
    title: 'Notifications on every device',
    text: 'Get mentions and direct messages on your computer and your phone. The inbox keeps what you missed.',
    shot: 'inbox',
    alt: 'The inbox with two direct messages and one mention.',
  },
  {
    title: 'Replies, reactions, edits and custom emoji',
    text: 'Use them on networks that support them. Other IRC clients see ordinary messages.',
    shot: 'reactions',
    alt: 'Messages with formatted text and emoji reactions.',
    fit: 'object-contain object-left-top',
  },
  {
    title: 'Your look',
    text: 'Choose from 13 color schemes, most of them in light and dark, and set the font and spacing.',
    shot: 'appearance',
    alt: 'Appearance settings with a preview and a grid of color schemes.',
  },
]
</script>

<template>
  <div>
    <!-- 1 Hero -->
    <section
      aria-labelledby="hero-title"
      class="hero overflow-hidden"
    >
      <div class="mx-auto max-w-6xl px-4 pt-16 pb-12 sm:px-6 sm:pt-24">
        <div class="max-w-3xl">
          <h1
            id="hero-title"
            class="text-[40px] leading-[1.05] font-bold tracking-[-0.03em] text-balance sm:text-[64px]"
          >
            IRC that stays connected.
          </h1>
          <p class="mt-6 max-w-2xl text-[18px] leading-relaxed text-muted sm:text-[20px]">
            {{ description }} Open it in a browser or install it on your phone.
          </p>
          <div class="mt-8 flex flex-wrap gap-3">
            <a
              :href="site.app"
              class="btn-primary"
              >Open IRCord</a
            >
            <a
              href="#host"
              class="btn-secondary"
              >Host your own</a
            >
          </div>
        </div>
        <div class="relative mt-14 sm:mt-16">
          <div
            class="overflow-hidden rounded-2xl border border-line shadow-2xl shadow-black/20 sm:rounded-3xl"
          >
            <AppShot
              name="channel-desktop"
              alt="IRCord in a browser: the #general channel with networks, channels, a conversation and the member list."
              eager
            />
          </div>
          <div
            class="mx-auto mt-6 w-[min(260px,70%)] overflow-hidden rounded-[28px] border-4 border-line shadow-2xl shadow-black/30 md:absolute md:right-[-12px] md:bottom-[-40px] md:mt-0 md:w-[200px] lg:right-[-32px] lg:w-[230px]"
          >
            <AppShot
              name="channel-phone"
              alt="The same channel on a phone."
              eager
            />
          </div>
        </div>
      </div>
    </section>

    <!-- 2 How it works -->
    <section
      id="how-it-works"
      aria-labelledby="how-title"
      class="scroll-mt-4 py-20"
    >
      <div class="mx-auto max-w-6xl px-4 sm:px-6">
        <p class="eyebrow">How it works</p>
        <h2
          id="how-title"
          class="mt-3 max-w-2xl text-[30px] leading-tight font-bold tracking-tight sm:text-[40px]"
        >
          A client in your browser, a bouncer on the server.
        </h2>
        <ol
          class="mt-10 grid items-stretch gap-3 md:grid-cols-[1fr_auto_1fr_auto_1fr]"
          aria-label="How your messages travel"
        >
          <li class="panel p-6">
            <h3 class="text-[18px] font-semibold">Your devices</h3>
            <p class="mt-2 text-muted">
              A browser or the installed app on your computer and phone.
            </p>
          </li>
          <li
            class="grid place-items-center text-link md:px-1"
            aria-hidden="true"
          >
            <svg
              class="size-6 rotate-90 md:rotate-0"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M4 8h15m-4-4 4 4-4 4M20 16H5m4-4-4 4 4 4" />
            </svg>
          </li>
          <li class="panel border-accent/40 p-6">
            <h3 class="text-[18px] font-semibold">IRCord server</h3>
            <p class="mt-2 text-muted">
              Stays connected for you and stores messages, files and settings.
            </p>
          </li>
          <li
            class="grid place-items-center text-link md:px-1"
            aria-hidden="true"
          >
            <svg
              class="size-6 rotate-90 md:rotate-0"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M4 8h15m-4-4 4 4-4 4M20 16H5m4-4-4 4 4 4" />
            </svg>
          </li>
          <li class="panel p-6">
            <h3 class="text-[18px] font-semibold">IRC networks</h3>
            <p class="mt-2 text-muted">Libera.Chat, OFTC, irc.ircord.com or your own server.</p>
          </li>
        </ol>
        <ul class="mt-10 grid gap-4 text-[17px] leading-relaxed md:grid-cols-3">
          <li>A bouncer is a server that stays on IRC for you. IRCord includes one.</li>
          <li>
            The server keeps your connections open and stores your messages, files and settings.
          </li>
          <li>
            Sign in on another device with the same username and password. Your conversations are
            already there.
          </li>
        </ul>
      </div>
    </section>

    <!-- 3 Features -->
    <section
      id="features"
      aria-labelledby="features-title"
      class="scroll-mt-4 bg-surface py-20"
    >
      <div class="mx-auto max-w-6xl px-4 sm:px-6">
        <p class="eyebrow">Features</p>
        <h2
          id="features-title"
          class="mt-3 text-[30px] leading-tight font-bold tracking-tight sm:text-[40px]"
        >
          What you can do with it
        </h2>
        <ul class="mt-10 grid gap-5 md:grid-cols-2">
          <li
            v-for="feature in features"
            :key="feature.title"
            class="flex flex-col overflow-hidden rounded-3xl border border-line bg-canvas"
          >
            <div class="p-6 pb-5">
              <h3 class="text-[19px] font-semibold">{{ feature.title }}</h3>
              <p class="mt-2 leading-relaxed text-muted">{{ feature.text }}</p>
            </div>
            <div class="mt-auto aspect-[16/10] overflow-hidden border-t border-line">
              <AppShot
                :name="feature.shot"
                :alt="feature.alt"
                :img-class="`block h-full w-full ${feature.fit ?? 'object-cover object-left-top'}`"
              />
            </div>
          </li>
        </ul>
      </div>
    </section>

    <!-- 4 IRC stays IRC -->
    <section
      aria-labelledby="irc-title"
      class="py-20"
    >
      <div class="mx-auto grid max-w-6xl gap-10 px-4 sm:px-6 md:grid-cols-[2fr_3fr]">
        <div>
          <p class="eyebrow">IRC stays IRC</p>
          <h2
            id="irc-title"
            class="mt-3 text-[30px] leading-tight font-bold tracking-tight sm:text-[40px]"
          >
            Ordinary IRC on the network
          </h2>
        </div>
        <ul class="grid gap-5 text-[17px] leading-relaxed">
          <li>
            IRCord connects to ordinary IRC networks, such as Libera.Chat and OFTC, and to your own
            server.
          </li>
          <li>People on irssi, WeeChat or any other client see ordinary messages.</li>
          <li>IRCord uses a network’s IRCv3 features only when the network offers them.</li>
          <li>
            Your IRCord account on chat.ircord.com is also your account on irc.ircord.com. The same
            sign-in works from any IRC client.
          </li>
        </ul>
      </div>
    </section>

    <!-- 5 Two ways to use it -->
    <section
      id="host"
      aria-labelledby="ways-title"
      class="scroll-mt-4 bg-surface py-20"
    >
      <div class="mx-auto max-w-6xl px-4 sm:px-6">
        <p class="eyebrow">Get started</p>
        <h2
          id="ways-title"
          class="mt-3 text-[30px] leading-tight font-bold tracking-tight sm:text-[40px]"
        >
          Two ways to use it
        </h2>
        <div class="mt-10 grid gap-5 md:grid-cols-2">
          <div class="flex min-w-0 flex-col rounded-3xl border border-line bg-canvas p-6 sm:p-7">
            <h3 class="text-[22px] font-semibold">Use chat.ircord.com</h3>
            <p class="mt-3 leading-relaxed text-muted">
              Create an account on the project’s own server. It comes with an account on the
              irc.ircord.com network. Add other networks at any time.
            </p>
            <p class="mt-3 leading-relaxed text-muted">
              Sign-up needs an invitation key for now. To ask for one,
              <a
                :href="site.newIssue"
                class="prose-link"
                >open an issue on GitHub</a
              >.
            </p>
            <div class="mt-auto flex flex-wrap gap-3 pt-6">
              <a
                :href="site.signup"
                class="btn-primary"
                >Create an account</a
              >
              <a
                :href="site.app"
                class="btn-secondary"
                >Sign in</a
              >
            </div>
          </div>
          <div class="flex min-w-0 flex-col rounded-3xl border border-line bg-canvas p-6 sm:p-7">
            <h3 class="text-[22px] font-semibold">Host your own</h3>
            <p class="mt-3 leading-relaxed text-muted">
              Run IRCord on your own server with Docker. It needs one data volume and no separate
              database.
            </p>
            <pre
              class="mt-5 overflow-x-auto rounded-2xl border border-line bg-surface p-4 font-mono text-[14px] leading-relaxed"
            ><code>git clone https://github.com/karbowiak/ircord.git
cd ircord
docker compose up -d --build</code></pre>
            <div class="mt-auto flex flex-wrap gap-3 pt-6">
              <a
                :href="site.docker"
                class="btn-secondary"
                >Read the setup guide</a
              >
            </div>
          </div>
        </div>
        <div
          id="instance"
          class="mt-5 rounded-3xl border border-line p-7"
          aria-labelledby="instance-title"
          role="region"
        >
          <h3
            id="instance-title"
            class="text-[18px] font-semibold"
          >
            About chat.ircord.com
          </h3>
          <ul
            class="mt-3 grid list-disc gap-2 pl-5 leading-relaxed text-muted marker:text-line md:grid-cols-2 md:gap-x-10"
          >
            <li>The IRCord project runs chat.ircord.com and irc.ircord.com.</li>
            <li>
              The server stores your messages, files and settings so they are there on every device.
            </li>
            <li>Delete your account at any time in Settings → Account.</li>
            <li>Accounts are deleted after 90 days without a visit.</li>
          </ul>
        </div>
      </div>
    </section>

    <!-- 6 For server owners -->
    <section
      aria-labelledby="owners-title"
      class="py-20"
    >
      <div class="mx-auto grid max-w-6xl gap-10 px-4 sm:px-6 md:grid-cols-[2fr_3fr]">
        <div>
          <p class="eyebrow">For server owners</p>
          <h2
            id="owners-title"
            class="mt-3 text-[30px] leading-tight font-bold tracking-tight sm:text-[40px]"
          >
            Run it for your community
          </h2>
        </div>
        <div>
          <ul class="grid gap-5 text-[17px] leading-relaxed">
            <li>
              Connect IRCord to your own Ergo IRC server. One sign-up then creates both accounts.
            </li>
            <li>Choose the channels new members are offered when they join.</li>
            <li>See the server’s health and add other owners in the Administration panel.</li>
            <li>Mark bots and remove other people’s messages from the conversation.</li>
          </ul>
          <p class="mt-6">
            <a
              :href="site.ergo"
              class="prose-link"
              >Connect an IRC server</a
            >
          </p>
        </div>
      </div>
    </section>

    <!-- 7 Open source -->
    <section
      aria-labelledby="source-title"
      class="pb-20"
    >
      <div class="mx-auto max-w-6xl px-4 sm:px-6">
        <div class="hero-panel panel p-8 sm:p-10">
          <p class="eyebrow">Open source</p>
          <h2
            id="source-title"
            class="mt-3 text-[30px] leading-tight font-bold tracking-tight sm:text-[40px]"
          >
            Built in the open
          </h2>
          <p class="mt-4 max-w-2xl text-[17px] leading-relaxed text-muted">
            IRCord is open source under the
            <a
              :href="site.license"
              class="prose-link"
              >MIT license</a
            >.
            <template v-if="latest">
              The latest release is {{ latest.version
              }}<template v-if="latest.date">, from {{ formatDate(latest.date) }}</template
              >.
            </template>
          </p>
          <p
            v-if="collaborators.length"
            class="mt-3 text-[17px] leading-relaxed text-muted"
          >
            Built by {{ names }}.
          </p>
          <div class="mt-7 flex flex-wrap gap-3">
            <NuxtLink
              to="/changelog"
              class="btn-primary"
              >What’s new</NuxtLink
            >
            <a
              :href="site.repo"
              class="btn-secondary"
              >Source code on GitHub</a
            >
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* Hibiki's ambient glow at the top of the page. */
.hero {
  background: radial-gradient(70% 55% at 50% 0%, var(--glow), transparent 70%);
}
.hero-panel {
  background: radial-gradient(60% 80% at 0% 0%, var(--glow), transparent 70%), var(--surface);
}
</style>
