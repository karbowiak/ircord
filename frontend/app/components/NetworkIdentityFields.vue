<script setup lang="ts">
/** Network dialog → Identity: nicknames, username and real name. */
const {
  form,
  standardNick,
  alternateNick,
  nickMax,
  nickProblem,
  alternateProblem,
  firstConnection,
} = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>Identity</legend>
    <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
      <AppField
        v-if="!firstConnection"
        id="network-nick"
        label="Nickname"
        :help="form.nick && nickProblem ? nickProblem : 'How people mention you. No spaces.'"
        :help-class="{ 'danger-text': form.nick && nickProblem }"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            :ref="(el) => (standardNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
            v-model="form.nick"
            required
            :maxlength="nickMax"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            :aria-invalid="!!form.nick && !!nickProblem"
          />
        </template>
      </AppField>
      <AppField
        id="network-alt"
        label="If taken, use"
        :help="alternateProblem || undefined"
        help-class="danger-text"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            :ref="(el) => (alternateNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
            v-model="form.altNick"
            :placeholder="`${form.nick || 'nick'}_`"
            :maxlength="nickMax"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            :aria-invalid="!!alternateProblem"
          />
        </template>
      </AppField>
      <AppInput
        id="network-user"
        v-model="form.username"
        label="Username"
        :placeholder="form.nick || 'username'"
        maxlength="32"
        autocomplete="off"
      />
      <AppInput
        v-if="!firstConnection"
        id="network-real"
        v-model="form.realname"
        label="Real name"
        help="Shown in chat and WHOIS. Spaces are allowed."
        placeholder="Your nickname when blank"
        maxlength="100"
        autocomplete="off"
      />
    </div>
  </fieldset>
</template>
