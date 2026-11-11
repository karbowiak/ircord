import type { Router } from 'vue-router'
import { createViewHistory, type ViewState } from '../../utils/view-history'

const stacks = new WeakMap<Router, ReturnType<typeof createViewHistory>>()

/** One shared lifecycle for dialogs, local views, browser Back and Forward. */
export function useDialogStack() {
  const router = useRouter()
  let stack = stacks.get(router)
  if (!stack) {
    stack = createViewHistory(router)
    stacks.set(router, stack)
    useNuxtApp().vueApp.onUnmount(stack.dispose)
  }
  return stack
}

/** Track navigation identifiers, never search text, drafts, form fields or credentials. */
export function useViewHistory<T extends ViewState>(key: string, state: Ref<T>) {
  const stop = useDialogStack().track(key, state)
  onScopeDispose(stop)
}
