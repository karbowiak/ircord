import { resolve, relative, isAbsolute } from 'node:path'
import { addTemplate, defineNuxtModule, updateTemplates } from '@nuxt/kit'
import { discoverEmbeds, embedManifest } from './embed-declarations'

export default defineNuxtModule({
  meta: { name: 'ircord-embeds' },
  setup(_options, nuxt) {
    const directory = resolve(nuxt.options.srcDir, 'components/embed')
    const types = resolve(nuxt.options.rootDir, 'shared/types/embed')
    const template = addTemplate({
      filename: 'embed-manifest.ts',
      write: true,
      getContents: async () => embedManifest(await discoverEmbeds(directory), directory, types),
    })
    nuxt.options.alias['#embed-manifest'] = template.dst
    nuxt.hook('builder:watch', async (_event, path) => {
      const file = isAbsolute(path) ? path : resolve(nuxt.options.srcDir, path)
      const local = relative(directory, file)
      if (!local.startsWith('..') && !isAbsolute(local) && local.endsWith('.vue'))
        await updateTemplates({ filter: (entry) => entry.dst === template.dst })
    })
  },
})
