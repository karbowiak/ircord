export type ProfileFields = { name: string; bio: string; color: string | null }
export type ProfileFieldProtection = Partial<Record<keyof ProfileFields, boolean>>

export function profileChanges(base: ProfileFields, draft: ProfileFields) {
  return {
    ...(draft.name.trim() !== base.name ? { displayName: draft.name.trim() } : {}),
    ...(draft.bio !== base.bio ? { bio: draft.bio } : {}),
    ...(draft.color !== base.color ? { nameColor: draft.color } : {}),
  }
}

// Advance the authoritative baseline, retaining fields edited in this form.
export function mergeProfileFields(
  base: ProfileFields,
  draft: ProfileFields,
  latest: ProfileFields,
  protect: ProfileFieldProtection = {},
  accepted: ProfileFieldProtection = {},
): ProfileFields {
  return {
    name:
      protect.name || (!accepted.name && draft.name.trim() !== base.name)
        ? draft.name
        : latest.name,
    bio: protect.bio || (!accepted.bio && draft.bio !== base.bio) ? draft.bio : latest.bio,
    color:
      protect.color || (!accepted.color && draft.color !== base.color) ? draft.color : latest.color,
  }
}
