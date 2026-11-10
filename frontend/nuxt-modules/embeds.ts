import { resolve, relative, isAbsolute } from 'node:path'
import { addTemplate, defineNuxtModule } from '@nuxt/kit'
import { discoverEmbeds, embedManifest } from './embed-declarations'

export default defineNuxtModule({
  meta: { name: 'ircord-embeds' },
  setup(_options, nuxt) {
    const directory = resolve(nuxt.options.srcDir, 'components/embed')
    const types = resolve(nuxt.options.rootDir, 'shared/types/embed')
    const template = addTemplate({
      filename: 'embed-manifest.ts',
      write: true,
      dependsOn: ({ path }) => {
        const local = relative(directory, path)
        return !local.startsWith('..') && !isAbsolute(local) && local.endsWith('.vue')
      },
      getContents: async () => embedManifest(await discoverEmbeds(directory), directory, types),
    })
    nuxt.options.alias['#embed-manifest'] = template.dst
  },
})
