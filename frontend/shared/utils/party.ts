import type { components } from '../generated/api'

type Party = components['schemas']['Party']

/** Names are account-based and remain available when participants are offline. */
export function partyName(party: Party, accountId: string | undefined, fallback: string) {
  return (
    party.name.trim() ||
    party.members
      .filter((member) => (member.backendAccountId || member.accountId) !== accountId)
      .map((member) => member.displayName || member.username)
      .join(', ') ||
    fallback
  )
}
