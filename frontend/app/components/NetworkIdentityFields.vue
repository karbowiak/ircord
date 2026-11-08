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
      <div v-if="!firstConnection">
        <FieldLabel for="network-nick">Nickname</FieldLabel
        ><UiInput
          id="network-nick"
          :ref="(el) => (standardNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
          v-model="form.nick"
          required
          :maxlength="nickMax"
          autocomplete="off"
          autocapitalize="none"
          spellcheck="false"
          :aria-invalid="!!form.nick && !!nickProblem"
          aria-describedby="network-nick-hint"
        />
        <FieldHint
          id="network-nick-hint"
          :class="{ 'danger-text': form.nick && nickProblem }"
        >
          {{ form.nick && nickProblem ? nickProblem : 'How people mention you. No spaces.' }}
        </FieldHint>
      </div>
      <div>
        <FieldLabel for="network-alt">If taken, use</FieldLabel
        ><UiInput
          id="network-alt"
          :ref="(el) => (alternateNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
          v-model="form.altNick"
          :placeholder="`${form.nick || 'nick'}_`"
          :maxlength="nickMax"
          autocomplete="off"
          autocapitalize="none"
          spellcheck="false"
          :aria-invalid="!!alternateProblem"
          aria-describedby="network-alt-hint"
        />
        <FieldHint
          v-if="alternateProblem"
          id="network-alt-hint"
          class="danger-text"
        >
          {{ alternateProblem }}
        </FieldHint>
      </div>
      <div>
        <FieldLabel for="network-user">Username</FieldLabel
        ><UiInput
          id="network-user"
          v-model="form.username"
          :placeholder="form.nick || 'username'"
          maxlength="32"
          autocomplete="off"
        />
      </div>
      <div v-if="!firstConnection">
        <FieldLabel for="network-real">Real name</FieldLabel
        ><UiInput
          id="network-real"
          v-model="form.realname"
          placeholder="Your nickname when blank"
          maxlength="100"
          autocomplete="off"
        />
        <FieldHint> Shown in chat and WHOIS. Spaces are allowed. </FieldHint>
      </div>
    </div>
  </fieldset>
</template>
