/** Nickname syntax shared by connection setup, profile edits and slash commands. */
const syntax = /^[A-Za-z[\]\\`_^{}|][A-Za-z0-9[\]\\`_^{}|-]*$/
export function nicknameLimit(isupport?: Record<string, string>): number {
  const value = Number(isupport?.NICKLEN)
  return Number.isInteger(value) && value > 0 && value <= 128 ? value : 32
}
export type NicknameTranslator = (key: string, values?: Record<string, string | number>) => string

export function nicknameError(value: string, maximum = 32, translate?: NicknameTranslator): string {
  if (!value) return translate?.('nickname.required') ?? 'Enter a nickname.'
  if (/\s/.test(value))
    return (
      translate?.('nickname.spaces') ??
      'Nicknames cannot contain spaces. Your display name (Settings → Profile) can.'
    )
  if (!syntax.test(value))
    return (
      translate?.('nickname.syntax') ??
      'Use letters, numbers or IRC nickname symbols. Start with a letter or an IRC symbol.'
    )
  if (value.length > maximum)
    return (
      translate?.('nickname.tooLong', { maximum }) ??
      `Use ${maximum} characters or fewer for your nickname.`
    )
  return ''
}
