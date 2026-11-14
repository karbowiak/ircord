/** Identity and selection captured when an asynchronous conversation request starts. */
export interface GuardedSelection {
  account?: string
  network: string
  revision: number
}

/** Seed only the original account's result and activate it only if the user has not moved. */
export function guardedSelectionResult(
  original: GuardedSelection,
  current: GuardedSelection,
  resultNetwork: string,
) {
  const seed = current.account === original.account && resultNetwork === original.network
  return {
    seed,
    activate:
      seed && current.network === original.network && current.revision === original.revision,
  }
}
