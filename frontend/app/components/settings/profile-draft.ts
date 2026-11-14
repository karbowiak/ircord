export type ProfileFields = { name: string; bio: string; color: string | null }
export type ProfileFieldProtection = Partial<Record<keyof ProfileFields, boolean>>

export function profileChanges(base: ProfileFields, draft: ProfileFields) {
  return {
    ...(draft.name.trim() !== base.name ? { displayName: draft.name.trim() } : {}),
    ...(draft.bio !== base.bio ? { bio: draft.bio } : {}),
    ...(draft.color !== base.color ? { nameColor: draft.color } : {}),
  }
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

/** The fields of `draft` that differ from `base`, for any flat set of profile fields. */
export function fieldChanges<T extends object>(base: T, draft: T): Partial<T> {
  return Object.fromEntries(
    (Object.keys(draft) as (keyof T)[])
      .filter((key) => !same(draft[key], base[key]))
      .map((key) => [key, draft[key]]),
  ) as Partial<T>
}

/**
 * `mergeProfileFields` for any flat set of fields: a field edited here (or protected while a
 * save is out) keeps the draft; every other field takes the latest stored value.
 */
export function mergeFields<T extends object>(
  base: T,
  draft: T,
  latest: T,
  protect: Partial<Record<keyof T, boolean>> = {},
  accepted: Partial<Record<keyof T, boolean>> = {},
): T {
  const out = { ...latest }
  for (const key of Object.keys(latest) as (keyof T)[]) {
    if (protect[key] || (!accepted[key] && !same(draft[key], base[key]))) out[key] = draft[key]
  }
  return out
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
