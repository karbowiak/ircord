<script setup lang="ts">
import { FocusScope } from 'reka-ui'
import { layoutLimits } from '../stores/layout'

/**
 * Keep this boundary in VDOM so Reka's as-child slot receives the panel component
 * VNode directly and forwards transition hooks to its element instead of a VaporSlot.
 */
const { t } = useI18n()
const chat = useChatStore()
const layout = useLayoutStore()
const appearance = useAppearanceStore()
const actions = useWorkspaceActions()
const { mobileMembers, membersDocked, panelMotion, membersVisible, searching, toggleSearch } =
  useWorkspacePane()

function closeMembers(event: KeyboardEvent) {
  if (!mobileMembers.value || event.defaultPrevented) return
  event.preventDefault()
  event.stopPropagation()
  mobileMembers.value = false
}
</script>

<template>
  <Transition
    name="right-panel-slide"
    :css="panelMotion"
    @before-leave="(el) => (el as HTMLElement).setAttribute('inert', '')"
    @before-enter="(el) => (el as HTMLElement).removeAttribute('inert')"
  >
    <FocusScope
      v-if="searching || membersVisible || mobileMembers"
      :key="chat.activeId"
      as-child
      :present="mobileMembers"
      :trapped="mobileMembers"
      :loop="mobileMembers"
      @mount-auto-focus.prevent
      @unmount-auto-focus.prevent
      ><ConversationSearch
        v-if="searching && chat.active"
        :key="chat.active.id"
        :role="mobileMembers ? 'dialog' : undefined"
        :inert="!membersDocked && !mobileMembers"
        :aria-modal="mobileMembers || undefined"
        :conversation="chat.active"
        @close="toggleSearch" />
      <MemberSidebar
        v-else-if="membersVisible || mobileMembers"
        :role="mobileMembers ? 'dialog' : undefined"
        :inert="!membersDocked && !mobileMembers"
        :aria-modal="mobileMembers || undefined"
        @profile="actions.profile"
        @close="mobileMembers = false"
        @keydown.esc="closeMembers"
      >
        <ResizeHandle
          :label="t('conversation.resizeMembers')"
          direction="left"
          :value="layout.state.membersWidth"
          :min="layoutLimits.members.min"
          :max="layoutLimits.members.max"
          :default="layoutLimits.members.default"
          :scale="appearance.settings.interfaceScale / 100"
          @resize="layout.set('membersWidth', $event)"
        /> </MemberSidebar
    ></FocusScope>
  </Transition>
</template>
