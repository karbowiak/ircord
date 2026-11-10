import { defineAsyncComponent } from 'vue'
import type { Component } from 'vue'

/** Load adapter code ahead of scrolling without mounting its player or joining page Suspense. */
export function createEmbedRenderer(loader: () => Promise<Component>, placeholder: Component) {
  let pending: Promise<Component> | undefined
  const preload = () =>
    (pending ??= loader().catch((error) => {
      pending = undefined
      throw error
    }))
  return {
    preload,
    component: defineAsyncComponent({
      loader: preload,
      // A slow provider must only delay its own box, including during page navigation.
      suspensible: false,
      loadingComponent: placeholder,
      errorComponent: placeholder,
      delay: 0,
      timeout: 15_000,
    }),
  }
}
