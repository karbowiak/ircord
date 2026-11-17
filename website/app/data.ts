// The repository's changelog and collaborators, read when the site is built. The Dockerfile
// copies both files next to the website, at the same relative path.
import changelog from '../../CHANGELOG.md?raw'
import collaboratorsFile from '../../COLLABORATORS.md?raw'
import { parseChangelog, parseCollaborators } from './utils/changelog'

export const releases = parseChangelog(changelog)
export const latest = releases[0]
export const collaborators = parseCollaborators(collaboratorsFile)
