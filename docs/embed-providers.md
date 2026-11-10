# Embed provider sources and fixtures

This reference records reusable Iframely sources and URL fixtures for IRCord.
The inventory includes every tracked upstream file at the pinned revision.
It does not register providers or establish that their links still work.

## Contract

The source is [itteco/iframely](https://github.com/itteco/iframely/tree/962bf9287f2ab5880ff96d4ac796ebb62217be57), revision `962bf9287f2ab5880ff96d4ac796ebb62217be57`.
Iframely uses the MIT license, copyright 2013–2022 Itteco Software Corp.
The fixture manifest retains its complete license and copyright notice.
Retain these notices when distributing copied or adapted material.

The [fixture manifest](../frontend/shared/embed-fixtures/iframely.json) contains source paths, hashes, line numbers, test options, examples, and endpoint records.
The [importer](../frontend/scripts/import-iframely-fixtures.mjs) parses JavaScript with Babel.
It reads the upstream checkout as data and does not import its modules.
It does not run package scripts, callbacks, feeds, selectors, or provider requests.

| Inventory                 | Count | Meaning                                                                  |
| ------------------------- | ----: | ------------------------------------------------------------------------ |
| Tracked source files      |   365 | Complete source manifest, including runtime assets and documentation     |
| Parsed JavaScript files   |   305 | All JavaScript outside upstream browser assets                           |
| Domain JavaScript files   |   119 | Provider implementations and supporting modules                          |
| Domain templates          |     4 | 500px, Yandex Maps, Piktochart, and Pinterest                            |
| Domain path families      |    77 | Includes helper families; this is not a supported-provider count         |
| Link mixin files          |    41 | Includes hosted media, oEmbed, images, and document viewers              |
| Metadata mixin files      |    52 | Includes Open Graph, Twitter Cards, structured data, and canonical URLs  |
| Custom plugin files       |     9 | Includes icons, errors, query handling, and indexing policy              |
| System source files       |    22 | Includes parsing and the oEmbed endpoint registry                        |
| Validator source files    |    26 | Includes image, media, frame, and layout validation                      |
| Static test fixtures      |   395 | 393 distinct URLs across 93 source modules                               |
| Test option declarations  |    87 | Preserves `noFeeds`, skipped methods, skipped mixins, and dynamic inputs |
| Dynamic test declarations |     8 | Feed and selector results remain unexpanded                              |
| Other URL literals        |    96 | Includes source endpoints and canonical or redirect literals             |
| Commented URL examples    |   194 | Includes 53 commented test examples                                      |
| Derived URL expressions   |    37 | Canonical or redirect expressions remain unevaluated                     |
| Known oEmbed records      |   152 | Complete upstream registry; records can share a provider                 |

### Manifest fields

- `sources` inventories every file and classifies every domain path family.
- `fixtures` contains literal, active test entries with their exact upstream URLs.
- `testDeclarations` retains static test controls and marks function values as dynamic.
- `dynamicTests` records feed, page, selector, and callback inputs without expanding them.
- `examples` keeps other URL literals separate from active test fixtures.
- `commentedExamples` retains commented URLs, nearby context, and available skip reasons.
- `derivedUrls` records canonical and redirect expressions that require runtime values.
- `oembedProviders` retains all known endpoint templates and URL conversion rules.
- `reviewCandidates` selects 18 likely suitable links for initial visible review.

The review candidates describe likely subjects from source context.
Their content, availability, and suitability remain unverified until visible review.
No fixture in this source manifest has a verified live or dead status.
One upstream fixture contains a credential-like value inside a nested player URL.
The importer omits that URL and records its source location without the value.

### Runtime ownership

Provider components live in `frontend/app/components/embed/`.
Each component declares literal `embedProvider` metadata for matching, accepted results, frame sources, and layout.
The Nuxt provider module extracts those declarations without executing component scripts.
`useEmbed` owns provider selection, backend requests, caching, expiry, and offline behavior.

The inventory is reference data and does not change runtime support.
A registry entry does not authorize its endpoint or iframe origin.
Only reviewed backend resolvers and matching frontend adapters can produce rich embeds.
Unknown or unsupported URLs retain the fixed metadata card.

Reserve the component's geometry before resolving a URL.
Keep loading, unavailable, media, and player states within that geometry.
Use native image, audio, or video elements when a reviewed public media URL is available.

Do not copy upstream user-agent overrides, cookies, private fetches, or arbitrary scripts into the shared framework.
Do not use upstream resizing behavior to change timeline card dimensions after loading.

### Runtime adapter geometry

The component directory includes controlled provider adapters, direct files and the fallback card.
These declarations describe allowed rendering and reserved space; they do not establish that every historical fixture works.
The timeline selects this space from the posted URL or trusted upload metadata before fetching a preview.

| Adapters or URL variant                                         | Reserved timeline geometry |
| --------------------------------------------------------------- | -------------------------- |
| YouTube, Vimeo, Dailymotion, Loom, Wistia, TED, Twitch          | 16:9                       |
| Apple Music music videos                                        | 16:9                       |
| Apple Music songs, including album URLs with a song selected    | 175px high                 |
| Apple Music albums and playlists                                | 450px high                 |
| Spotify tracks and episodes                                     | 152px high                 |
| Other supported Spotify collections                             | 352px high                 |
| SoundCloud tracks                                               | 166px high                 |
| SoundCloud sets                                                 | 450px high                 |
| Bandcamp, Mixcloud                                              | 120px high                 |
| ART19, Simplecast                                               | 200px high                 |
| Libsyn                                                          | 96px high                  |
| Deezer                                                          | 300px high                 |
| CodePen, JSFiddle, LiveCodes                                    | 400px high                 |
| SpeakerDeck, SlideShare, GeoGebra                               | 4:3                        |
| DocumentCloud, Scribd                                           | 3:4                        |
| Prezi                                                           | 11:8                       |
| Google documents and Drive readers                              | 3:4                        |
| Google spreadsheets, drawings, calendars and shared maps        | 4:3                        |
| Google presentations                                            | 16:9                       |
| Google Forms                                                    | 640px high                 |
| Slides presentations                                            | 48:35                      |
| Quizlet                                                         | 500px high                 |
| OpenStreetMap and Yandex Maps                                   | 4:3                        |
| Padlet                                                          | 608px high                 |
| DBDiagram                                                       | 16:9                       |
| Flickr, XKCD, Droplr photos, ImageShack, SmugMug, direct images | Fitted image, 400px cap    |
| Direct videos, Droplr videos, Giphy clips, upload galleries     | 400px high                 |
| Giphy, direct GIFs                                              | Fitted image, 200px cap    |
| Reddit, Instagram posts                                         | 420px high                 |
| Instagram reels and videos                                      | 500px high                 |
| TikTok videos                                                   | 578px high                 |
| Direct audio                                                    | 88px high                  |
| Files, unknown URLs                                             | 150px high                 |

Players, cards and galleries use the available width up to 480px.
Image adapters can grow wider, preserving their initial validated aspect ratio within the chat column and height cap.
Without initial dimensions, an image keeps its fixed-height reservation for that mounted instance.
Player aspect ratios follow their width; fixed pixel heights remain fixed on narrow screens.
Loading, consent, expiry, unsupported-result, and error states retain the selected geometry.
Metadata prefetching extends farther than player activation. Each link observes its own box.
Only adapters within 120 pixels of the visible chat mount their actual player/media contents.
Distant boxes keep cached details without retaining off-screen frames or decoders.
An open media viewer remains mounted.
Validated image dimensions only determine the inner media rectangle and its controls.
They cannot resize the reserved timeline row or select a different adapter after fetching.

Giphy, Flickr, and XKCD can use a single validated image from IRCord's media cache.
Droplr photos, ImageShack and SmugMug use the same validated native-image path.
Droplr videos and Giphy clips require their reviewed video routes and retain fixed geometry.
Their source policy does not authorize arbitrary oEmbed photos or additional images from unrelated sites.
Each external URL keeps its own preview; an album supplied by one Reddit post and an explicit uploaded file batch are separate gallery cases.

## Defaults

### Reusable domain families

These categories describe upstream source requirements, not current IRCord support.
Counts include templates and supporting files within each family.
The manifest records each file and its specific methods, mixins, matching rules, and options.

| Source category                       | Files | Domain path families                                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------------- | ----: | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Standard oEmbed or player adaptation  |    22 | `art19.com`, `cartodb.com`, `cnevids.com`, `codepen.io`, `documentcloud.org`, `issuu.com`, `itunes.apple.com`, `livecodes.io`, `loom.com`, `mixcloud.com`, `polldaddy.com`, `slideshare.net`, `soundcloud.com`, `speakerdeck.com`, `spotify.com`, `ted.com`, `vimeo.com`, `widgetic.com`, `wistia.com`                                                                                                            |
| Native photo or media adaptation      |    11 | `500px`, `d.pr`, `flickr.com`, `giphy.com`, `i.giphy.com`, `imageshack.com`, `smugmug.com`, `xkcd.com`                                                                                                                                                                                                                                                                                                            |
| Controlled custom iframe adaptation   |    38 | `abcnews.com`, `cbc.ca`, `dbdiagram.io`, `espn.com`, `geogebra.org`, `google.com`, `hockeydb.com`, `imdb.com`, `jsfiddle.net`, `knightlab`, `libsyn.com`, `maps.yandex.ru`, `momindum.com`, `openstreetmap.org`, `padlet.com`, `pastebin.com`, `piktochart.com`, `prezi.com`, `quizlet.com`, `readymag.com`, `simplecast.com`, `slid.es`, `twitch.tv`, `v.qq.com`, `vbox7.com`, `washingtonpost.com`, `youku.com` |
| Custom HTML or metadata extraction    |    23 | `bandcamp.com`, `buzzfeed.com`, `dailymail.com`, `github.gist`, `kickstarter.com`, `mail.ru`, `npr.org`, `nytimes.video`, `pinterest.com`, `rapgenius.com`, `tmz.com`, `twitter.com`, `usatoday.com`                                                                                                                                                                                                              |
| Credentials or private upstream paths |    25 | `dailymotion.com`, `facebook.com`, `instagram.com`, `scribd.com`, `tumblr.com`, `youtube.com`                                                                                                                                                                                                                                                                                                                     |
| Routing or policy helpers             |     4 | `ignore`, `spotify.com-meta-fallback`, `usatoday-allowEmbedURL`, `vimeo-moogaloop`                                                                                                                                                                                                                                                                                                                                |

The first useful families are public players, audio widgets, code demos, presentations, and native photos or GIFs.
Generic oEmbed can supply many of these results without another Go extractor.
Custom URL conversions can still require a small reviewed resolver or adapter.

The known endpoint registry also covers providers without domain plugins.
Examples include Deezer, Apple Podcasts, Reddit, Datawrapper, Figma, Canva, Miro, Airtable, and many others.
All 152 records remain in the manifest for review, including historical and credential-dependent entries.
Do not treat this static registry as a current service directory.

### Generic source families

| Source area                                                     | Reusable information                                                        | Required boundary                                                             |
| --------------------------------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `plugins/links/oembed-*`                                        | Photo URLs, iframe extraction, thumbnails, and rich or video classification | Validate the selected result and allowed source; do not inject arbitrary HTML |
| `plugins/links/og-*`, `twitter-*`, `image_src`, and `video_src` | Public metadata and media candidates                                        | Validate URLs and content types before native rendering                       |
| `plugins/links/hosted/`                                         | Known hosters, canonical redirects, and podcast players                     | Review each host and route before allowing a player                           |
| `plugins/links/embedURL/`                                       | Structured-data player URLs                                                 | Keep destination validation and provider policy in the backend                |
| `plugins/links/*-doc-viewer`                                    | Public document viewer conversions                                          | Review third-party document exposure and fixed layout                         |
| `plugins/meta/`                                                 | Titles, authors, descriptions, dates, and canonical URLs                    | Retain backend metadata ownership and account isolation                       |
| `plugins/custom/`                                               | Error mapping, icons, and policy helpers                                    | Adapt behavior selectively; retain IRCord's error and credential rules        |
| `lib/plugins/system/oembed/`                                    | Discovery, known endpoints, and URL normalization                           | Adopt endpoint records individually and validate returned player origins      |
| `lib/plugins/validators/`                                       | Media checks and upstream failure cases                                     | Keep bounded requests and fixed geometry; exclude dynamic resizing            |

### Refresh the manifest

1. Keep the pinned upstream checkout under the ignored `.data/references/iframely` directory.
2. Run `node scripts/import-iframely-fixtures.mjs` from `frontend/`.
3. Run `node scripts/import-iframely-fixtures.mjs --check` to verify deterministic output.
4. Review source, count, fixture, and license changes before adopting another upstream revision.

The importer requires a clean reference checkout at the exact pinned revision.
It accepts `--source=PATH`, resolved relative to `frontend/`, for another local checkout.
It does not fetch or update that checkout.
The fixture JSON is maintained by the importer; do not hand-edit it.

### Build a visible review queue

Run `bun scripts/build-embed-review-queue.mjs` from `frontend/` to create the ignored `.data/embed-review/queue.json` file.
The helper uses the actual static component extractor and verifies the existing Nuxt manifest.
It calls the application's `selectEmbed` function through a local Bun alias.
It does not execute component scripts, lazy loaders, or upstream provider code.

Each queue entry includes the exact source URL, selected component, fixed layout, and required input metadata.
Missing source fixtures and unmatched upstream routes remain explicit gaps.
Gallery entries require trusted attachment metadata and cannot be tested by posting their URL alone.
The queue contains pending cases, not browser test results.
Record visible review results separately from the tracked fixture catalog.

## Failure modes

### Dynamic and missing tests

Eight active declarations depend on feeds or selectors.
They cover Bandcamp, Flickr galleries, Flickr photos, Gist discovery, ImageShack, TED, Vimeo, and XKCD.
The manifest preserves their inputs and marks them unexpanded.
The Dailymotion callback test is commented upstream and remains a commented example.

Some modules have no active literal tests.
Examples include the main Dailymotion player, Google Maps, Street View, Kickstarter, Widgetic, and several support modules.
Commented examples retain upstream notes such as login redirects, soft 404s, changed formats, or required player configuration.
These notes explain upstream exclusions; they do not prove a fixture's current state.

No link is marked dead from source inspection alone.
The verified-live and verified-dead counts are both zero because this extraction makes no network requests.
Run focused provider checks separately and record their actual results.
Do not expand feeds or post the whole fixture set into a channel.

### Credentials and restricted paths

Google readers require a successful anonymous response for the posted document, calendar or shared map.
The player must retain the same content identifier. Private documents and access-key links remain unavailable.
Calendar previews also require the calendar's public iCal feed.
Source checks cover 17 upstream Google fixtures; live browser review is separate.

| Upstream family                  | Restriction or difference                                                                                            |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| Dailymotion                      | The main plugin requires a partner Player ID unless oEmbed supplies one. Its main video tests are commented out.     |
| Facebook and Instagram           | Graph oEmbed and upstream access workarounds require separate credential and behavior review.                        |
| YouTube                          | Upstream video and channel metadata uses `youtube.api_key`. Public iframe URL conversion can remain independent.     |
| Tumblr                           | The upstream API mixin requires `tumblr.consumer_key`. Photo and video helpers depend on its result.                 |
| Google Maps and Street View      | API paths require owner-supplied keys. Public Docs, Calendar, and custom map routes are separate cases.              |
| Scribd                           | The error plugin retrieves private document access keys. Exclude this path when adapting public document embeds.     |
| Twitch                           | The player requires configured parent domains. Use the actual application host rather than fixture query parameters. |
| Twitter and Pinterest            | Upstream can return script-backed HTML. Use a reviewed isolated adapter before supporting that output.               |
| Gist and publisher video plugins | Some sources fetch or transform page HTML. Standard oEmbed support does not replace that work.                       |

During the initial browser review, the old Simplecast fixture returned a page shell without static player or oEmbed metadata,
the Dailymotion response supplied a legacy player URL without its required partner ID,
and the CodePen fixture returned HTTP 403.
These are fallback outcomes for those fixtures, not proof that all links from those providers are unavailable.
Do not invent identifiers, run provider scripts to extract restricted data, or bypass access controls to turn them into rich embeds.
Legacy numeric GeoGebra URLs also remain fallback cases; the controlled adapter supports the reviewed alphanumeric material routes.

The upstream whitelist directory contains only a README.
The full quality-assurance allowlist is hosted remotely and is absent from this revision.
Its advertised domain totals do not describe this source inventory.
The importer does not fetch or silently recreate that list.

Legacy names, Flash routes, old endpoint templates, and historical test URLs remain available for review.
They remain unverified and do not automatically enter the runtime resolver.
Provider-specific private or hosted code that is absent upstream cannot be extracted from this checkout.

## Source

- [Pinned upstream tree](https://github.com/itteco/iframely/tree/962bf9287f2ab5880ff96d4ac796ebb62217be57)
- [Upstream MIT license](https://github.com/itteco/iframely/blob/962bf9287f2ab5880ff96d4ac796ebb62217be57/LICENSE)
- [Upstream provider guidance](https://github.com/itteco/iframely/blob/962bf9287f2ab5880ff96d4ac796ebb62217be57/docs/PROVIDERS.md)
- [Known endpoint registry](https://github.com/itteco/iframely/blob/962bf9287f2ab5880ff96d4ac796ebb62217be57/lib/plugins/system/oembed/providers.json)
- [Test expansion behavior](https://github.com/itteco/iframely/blob/962bf9287f2ab5880ff96d4ac796ebb62217be57/modules/tests-ui/tester.js)
- [Whitelist loader](https://github.com/itteco/iframely/blob/962bf9287f2ab5880ff96d4ac796ebb62217be57/lib/whitelist.js)
- [Frontend boundaries](frontend.md)
