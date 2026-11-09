/** Nickname syntax shared by connection setup, profile edits and slash commands. */
const syntax = /^[A-Za-z[\]\\`_^{}|][A-Za-z0-9[\]\\`_^{}|-]*$/
export function nicknameLimit(isupport?: Record<string, string>): number {
  const value = Number(isupport?.NICKLEN)
  return Number.isInteger(value) && value > 0 && value <= 128 ? value : 32
}
export function nicknameError(value: string, maximum = 32): string {
  if (!value) return 'Enter a nickname.'
  if (/\s/.test(value))
    return 'Nicknames cannot contain spaces. Your display name (Settings → Profile) can.'
  if (!syntax.test(value))
    return 'Use letters, numbers or IRC nickname symbols. Start with a letter or an IRC symbol.'
  if (value.length > maximum) return `Use ${maximum} characters or fewer for your nickname.`
  return ''
}
