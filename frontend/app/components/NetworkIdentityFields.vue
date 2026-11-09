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
    <legend>{{ $t('networkForm.identity') }}</legend>
    <div
      class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start] max-[700px]:[grid-template-columns:1fr]"
    >
      <AppField
        v-if="!firstConnection"
        id="network-nick"
        :label="$t('networkForm.nickname')"
        :help="form.nick && nickProblem ? nickProblem : $t('networkForm.nicknameHelp')"
        :help-class="{ 'text-destructive': form.nick && nickProblem }"
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
        :label="$t('networkForm.alternateNick')"
        :help="alternateProblem || undefined"
        help-class="text-destructive"
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
        :label="$t('networkForm.username')"
        :placeholder="form.nick || 'username'"
        maxlength="32"
        autocomplete="off"
      />
      <AppInput
        v-if="!firstConnection"
        id="network-real"
        v-model="form.realname"
        :label="$t('networkForm.realName')"
        :help="$t('networkForm.realNameHelp')"
        :placeholder="$t('networkForm.realNamePlaceholder')"
        maxlength="100"
        autocomplete="off"
      />
    </div>
  </fieldset>
</template>
