import type { ComputedRef, InjectionKey, Ref } from 'vue'
import type { Message } from '../../../shared/types/chat'

interface WorkspacePane {
  title: ComputedRef<string | undefined>
  replyTo: Ref<Message | undefined>
  highlightId: Ref<string>
  composer: Ref<{ resume: () => boolean } | null>
  mobileNav: Ref<boolean>
  mobileMembers: Ref<boolean>
  membersDocked: ComputedRef<boolean>
  panelMotion: Ref<boolean>
  membersVisible: ComputedRef<boolean>
  membersOpen: ComputedRef<boolean>
  toggleMembers: () => void
  /** The conversation's search panel is open (in the member list's place). */
  searching: ComputedRef<boolean>
  toggleSearch: () => void
  requestReply: (message: Message) => void
  drop: ReturnType<typeof useFileDrop>
}
const paneKey: InjectionKey<WorkspacePane> = Symbol('workspace-pane')
export const provideWorkspacePane = (pane: WorkspacePane) => provide(paneKey, pane)
export function useWorkspacePane() {
  const pane = inject(paneKey)
  if (!pane) throw new Error('Conversation pages require the workspace shell.')
  return pane
}
