/**
 * The names listed in the repository's COLLABORATORS.md: one top-level bullet per person
 * (`- Name` or `* Name`). Headings, prose and nested bullets are ignored, and Markdown emphasis,
 * code marks and link targets are stripped so only the name is shown.
 */
export function parseCollaborators(markdown: string): string[] {
  const names: string[] = []
  for (const line of markdown.split(/\r?\n/)) {
    const item = /^[-*+]\s+(.+?)\s*$/.exec(line)
    if (!item) continue
    const name = item[1]!
      .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
      .replace(/[*_`]/g, '')
      .trim()
    if (name && !names.includes(name)) names.push(name)
  }
  return names
}
