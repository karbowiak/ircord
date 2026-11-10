// Name mentions (#209) need an “@”: “hey @alice” highlights alice, “Alice is here” does not.
// Plain words still highlight through highlight words, so someone who wants the old behavior
// adds their own nick there. Only your nick on the message's network counts; display names
// and account names are not aliases. The backend applies the same rule
// (backend/internal/notify/mentions.go); backend/internal/notify/testdata/mentions.json
// lists the cases both follow.

/** May come right before the “@”: anything else makes it an address or a path. */
const openers = '([{<"\'“‘«*,;'
// Go's unicode.IsSpace: unlike \s it includes U+0085 and leaves out U+FEFF.
const space = /^[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]$/u
// Go's unicode.IsLetter, IsDigit (decimal digits only) and IsMark.
const otherNickRune = /^[\p{L}\p{Nd}\p{M}]$/u
const letterOrDigit = /^[\p{L}\p{Nd}]$/u

/** A character a nick can hold: the RFC 1459 set, and letters, digits and marks of other scripts. */
function nickRune(c: string) {
  if (c.codePointAt(0)! < 0x80) return /^[a-zA-Z0-9\-[\]\\`^_{|}]$/.test(c)
  return otherNickRune.test(c)
}

/** Whether the text right after a candidate mention makes it part of a longer nick or address. */
function continuesNick(rest: string[]) {
  const c = rest[0]!
  if (nickRune(c) || c === '@') return true
  return c === '.' && rest.length > 1 && letterOrDigit.test(rest[1]!)
}

/** Whether the word that ends at the mention contains “://”. */
function inURL(before: string[]) {
  let start = before.length
  while (start > 0 && !space.test(before[start - 1]!)) start--
  return before.slice(start).join('').includes('://')
}

import { foldPersonIdentity } from './utils/people'

/** One character folded under the network's CASEMAPPING, like the backend's foldRune. */
const foldRune = foldPersonIdentity

/**
 * Whether text, with IRC formatting already removed, holds an explicit “@nick” mention. Nicks
 * compare under the network's CASEMAPPING (“ascii”, “rfc1459” when missing, “strict-rfc1459”,
 * anything else by Unicode lower case). The mention must be a whole nick: “@alice123”,
 * “@alice-” and “@alice.example” do not mention alice, and a mention inside a URL does not count.
 */
export function nickMentioned(text: string, nick: string | undefined, caseMapping = '') {
  if (!nick) return false
  const fold = (chars: string[]) => chars.map((c) => foldRune(c, caseMapping)).join('\u0000')
  const nickChars = [...nick]
  const want = fold(nickChars)
  const chars = [...text]
  for (const [i, c] of chars.entries()) {
    if (c !== '@') continue
    if (i > 0 && !space.test(chars[i - 1]!) && !openers.includes(chars[i - 1]!)) continue
    const end = i + 1 + nickChars.length
    if (end > chars.length || fold(chars.slice(i + 1, end)) !== want) continue
    if ((end < chars.length && continuesNick(chars.slice(end))) || inURL(chars.slice(0, i)))
      continue
    return true
  }
  return false
}
