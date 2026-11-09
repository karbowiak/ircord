# Frontend components and styling

This reference describes the frontend component boundaries and retained specialist styling.
Use it when changing shared controls, workspace layouts, or appearance behavior.

## Contract

`frontend/app/assets/css/tailwind.css` is the single stylesheet entry point.
Its layers are `theme`, `base`, `components`, `utilities`, and `specialist`, in that order.
The legacy layer and its control, dialog, and workspace files are removed.

`frontend/shared/appearance.ts` owns appearance values and semantic tokens.
The appearance store and pre-paint script apply saved values.
Unset appearance defaults to System, including before the application bundle loads.
Saved Light and Dark choices retain their explicit mode.
Keep this authority when generating or updating UI primitives.
Interface scale defaults to 100% with a 14px base and ranges from 85% to 150%.
Use the shared `--ui-*` type variables for navigation and shared controls.
Message text keeps its separate size under Chat. Interface scale follows the existing appearance sync and reset behavior.
Tailwind Preflight remains omitted; `base.css` supplies normalization and focus behavior.
Tooltips open on hover and keyboard focus. Restoring focus after a pointer click does not reopen a label.

### Browser request protection

`server/middleware/backend.ts` checks unsafe requests under `/api/app/` and `/api/v1/` before routing or proxying.
An `Origin` header must exactly match `NUXT_SITE_URL`, or the direct request origin when that setting is absent.
Fetch Metadata requests must identify a same-origin request and include a matching `Origin`.
Null, malformed, foreign, and sibling origins are rejected before account storage or backend calls.
Headerless clients remain supported. Safe reads, including isolated widgets and shared files, keep their existing authentication rules.
Direct backend access retains its separate Go checks; this frontend guard does not protect a separately exposed backend.

Set `NUXT_SITE_URL` to the browser-facing origin when a reverse proxy terminates HTTPS.
The check never trusts forwarded host or protocol headers.
Compose supplies this value from `IRCORD_PUBLIC_URL`; other deployments must set it explicitly.
These checks supplement session authentication and the existing embed isolation and Content Security Policy.

### Component inventory

| Surface                      | Owners and disposition                                                                                                                                                                                              |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shared controls              | `UiButton`, `UiInput`, `UiTextarea`, `UiLabel`, `UiCheckbox`, `UiSwitch`, `UiRadioGroup`, `UiSelect`, and `UiSlider` use the shared theme.                                                                          |
| Labels and validation        | `AppField` associates labels, help, and errors with controls. `AppInput` forwards native attributes and exposes focus. Rich hints use slots.                                                                        |
| Sign-in and registration     | `LoginScreen` uses shared fields. Its scroll area and `keep-field-in-view.client.ts` handle short screens and visual viewport changes.                                                                              |
| Loading and notices          | `BootScreen`, `EmptyState`, `UiAlert`, `UiBadge`, and `ConnectionBanners` own their presentation.                                                                                                                   |
| Settings navigation          | `SettingsLayout` and `SettingsNavItem` serve Settings, Administration, and Channel settings. Section navigation retains `aria-current`; it is not a tablist.                                                        |
| Settings content             | Shared titles, descriptions, groups, rows, notes, and links provide common presentation. `ChipInput` owns editable word and nickname lists.                                                                         |
| Network setup                | `NetworkDialog` composes Server, Identity, Login, and On connect fields. `useNetworkForm` owns shared form state.                                                                                                   |
| Channel settings             | `ChannelOverview`, `ChannelModes`, `ChannelModeList`, and `ChannelRegistration` retain separate responsibilities. `useChannelContext` supplies their context.                                                       |
| Notifications                | `NotificationDevice` and `NotificationRouting` split device setup from notification routing.                                                                                                                        |
| Dialogs and media viewer     | `AppDialog` and `MediaViewer` use Reka Dialog. `useDialogStack` coordinates dismissal and update/typing guards.                                                                                                     |
| Menus, selects, and tooltips | Context menus use Reka DropdownMenu. Desktop selects use Reka Select; touch selects remain native. `AppTooltip` shares the app-wide provider.                                                                       |
| Result views                 | `SearchField`, `ConversationResult`, and `MessageResult` serve Search, Saved, Inbox, and Links. Search, Inbox, and server-detail tabs use Reka Tabs.                                                                |
| Links                        | `FilterMenu` owns the channel, network and type dropdowns. `LinkCatalogCard` owns each resource card, its preview and its posts menu.                                                                               |
| Navigation                   | `WorkspaceBar` owns network tabs, account controls, and global search. `ConversationSidebar`, `ConversationLink`, `ChatHeader`, and `MemberSidebar` own their presentation. Docking follows `workspace-layout.css`. |
| Timeline                     | `ChatTimeline`, `MessageItem`, `EventRow`, `MessageExtras`, and attachment components own their markup. `timeline.css` coordinates layout modes and row states.                                                     |
| Composer                     | `ChatComposer`, `ComposerSuggestions`, and `ComposerUploads` retain native editing, completion, and upload behavior with component utilities.                                                                       |
| Server window                | `ServerWindow` owns the log; `ServerOverview` the status, latency, security and session cards; `ServerCapabilities` the capability and ISUPPORT column. Latency is polled, not pushed.                              |
| Identity and media           | Existing avatar, identity-color, attachment, and link-media components retain their loading, permission, and playback behavior.                                                                                     |

### Retained custom controls

- `AccountCombobox` allows free text. Enter submits that text unless a suggestion is highlighted.
- `AdminAccountPicker` displays results inline rather than in a popup.
- The expression picker supports resizing, viewport clamping, and a phone sheet layout.
- Touch controls use at least 44 pixels for primary buttons. Message actions remain visible on touch tablets.
- Navigation and member drawers contain keyboard focus and return it when closed.
- Foldable layouts keep history, composition, settings, and expression pickers clear of physical hinges.
- The composer measures draft wrapping after controls appear and when its width changes.
- When its panel is at most 1050 pixels wide, Server shows its log below the overview cards and keeps Capabilities in a separate view.
- Composer completion preserves the textarea caret, input method composition, and native editing.
- Message action buttons retain native titles, including explanations for disabled actions.
- File and color inputs retain their native controls.
- Navigation rows, avatar controls, resize handles, and game controls retain their domain-specific native markup.

### Stylesheet ownership

| File                                        | Purpose                                                                                                   |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `fonts.css`, `base.css`, `theme.css`        | Font stacks processed by Nuxt Fonts, normalization, global geometry, focus, and semantic utility tokens.  |
| `identity.css`                              | Shared identity tone classes.                                                                             |
| `controls.css`                              | Native select and file-input styling, including the phone font-size minimum.                              |
| `settings.css`                              | Settings row typography and shared channel lists and facts. Component utilities can override these rules. |
| `workspace-layout.css`                      | Panel docking, collapsed columns, drawer state, safe areas, and viewport segments across components.      |
| `timeline.css`                              | Cozy, Compact, and IRC layout relationships; row tint precedence; reply connectors; toolbar containment.  |
| `message-format.css`                        | Generated IRC formatting, lists, code, and syntax highlighting.                                           |
| `link-media.css`                            | Media renderers, measured preview geometry, and playback states.                                          |
| `expressions.css`                           | Expression picker grids, inline emoji, resize behavior, and phone sheet layout.                           |
| `presence.css`                              | Presence variants, shared animations, typing indicators, and reduced-motion handling.                     |
| `appearance-previews.css`, `hash-field.css` | Appearance preview artwork and the Hash Field game.                                                       |

Specialist rules follow utilities because their relationships must override each component's base presentation.
Keep new ordinary spacing, type, and control states in the component that owns them.
Preserve stable class hooks used by interaction code and custom theming.

### Workspace pages

Nuxt owns page routing and browser history. The shared workspace stays mounted while its right pane changes.
`app/app.vue` uses `NuxtLayout` with the default workspace layout. Authentication gates the workspace in `app/layouts/default.vue`.
`AppWorkspace` owns navigation, dialogs, and shared conversation actions. `WorkspacePage` supplies the common page header, description, count, and return action.
`WorkspaceToolbar`, `WorkspaceFooter`, and `WorkspaceLoading` share filter layout, keyboard hints, and loading feedback.
Page headers, filters, and results align to a centered 1080px reading width. Result lists scroll independently of their tools.
`MessageResult` supplies author avatars, source and timestamp metadata, reason labels, and bounded multiline previews.
Links use two columns on wide desktops, with host and date controls in the Filters disclosure.
Server keeps its full-width log and composer. Connection details and capabilities open from its summary row.

| Address                                            | Content                                                                                 |
| -------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `/channel/:id`                                     | Channel timeline, composer, and member list.                                            |
| `/dm/:id`                                          | Direct-message timeline and composer.                                                   |
| `/server/:id`                                      | Server window for the selected network.                                                 |
| `/search`, `/inbox`, `/saved`, `/drafts`, `/links` | Full-pane workspace tools.                                                              |
| `/party`                                           | Reserved for future group conversations; no page or group conversation type exists yet. |

Conversation routes share `ConversationPane`. Identifiers are account-scoped conversation or network identifiers.
Bare conversation addresses select a matching conversation on the current network, or show an empty state.
The root address selects the current conversation. Existing network/channel hash links redirect to the corresponding page address.
`useUrlSync` coordinates route changes and store selection. `useConversationNavigation` supplies canonical destinations.
Settings, profiles, network setup, channel settings, and administration remain dialogs.
Utility pages keep their filters when revisited. Opening a result returns to its conversation and preserves drafts.
Hidden conversations do not become read while a workspace tool is open.

Pages, dialogs, pickers, and code-highlighting grammars load with the app.
The `pages:extend` hook sets Nuxt's synchronous page flag, `_sync`, for the pinned Nuxt version.
Keep that hook covered by a production build when upgrading Nuxt.
Run `bun run build` and `bun run test:workspace` from `frontend/` to verify page navigation in an isolated production demo.

`DefaultAvatarPicker` uses the shared Reka radio group in registration and Profile settings.
Suggested and selected pictures retain distinct outlines. Upload actions use shared buttons.

`ReadAloudSettings` uses shared fields, notes, rows, and buttons.
Its device switch and speech-service key remain local; service configuration follows the account.

Group-mention policy lives in `ChannelOverview` and uses `AppSegmented` with operator permission checks.
Group autocomplete uses the same option layout as personal mentions.
`MessageQuoteBlock` owns quote presentation and composes the existing code, list, and text renderers.

### Nuxt modules

| Integration                                             | Ownership                                                                                    |
| ------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Built-in head metadata, route headers, and `robots.txt` | Page-specific public link cards, crawl refusal, and indexing headers.                        |
| `@nuxt/fonts`                                           | Fontsource assets, Unicode ranges, and adjusted local fallback metrics.                      |
| `@nuxt/image`                                           | Avatar, link-card, and gallery-thumbnail presentation through the `ircord` provider.         |
| `@vueuse/nuxt`                                          | Auto-imported lifecycle helpers; the existing stores and query cache retain their ownership. |
| `@nuxt/eslint`                                          | Nuxt-aware Vue and TypeScript correctness rules. Prettier owns formatting.                   |
| `@nuxt/a11y`                                            | Development accessibility feedback through DevTools.                                         |
| `@nuxtjs/i18n`                                          | English UI catalogs, translator adapters, and locale-aware formatting.                       |

The app remains client rendered. `shared/page-metadata.ts` defines fixed public route categories and English fallbacks.
The Nitro `render:html` hook emits page titles, descriptions, the static logo, and safe canonical URLs in the initial HTML.
A client plugin updates public tags during navigation. Signed-in browser titles and unread indicators have a separate owner.
Set `NUXT_SITE_URL` to the public origin behind a reverse proxy.
Public metadata falls back to the forwarded request origin; the proxy must replace untrusted forwarding headers.
Browser request protection uses the stricter origin rules described above.
Conversation identifiers, queries, channel names, messages, accounts, and member data never enter public tags.
Only fixed route labels such as `/channel` and `/dm` enter canonical URLs.
All workspace pages use `noindex, nofollow`, and `robots.txt` disallows crawling.
Nuxt head metadata, an `X-Robots-Tag` response header, and a static `public/robots.txt` provide this policy without a Robots module.
These directives do not provide access control. There is no sitemap or chat-derived structured data.

Font family names and saved appearance keys stay stable.
`fonts.css` is the single source of literal family stacks so Nuxt Fonts can add metric-adjusted fallbacks.
The appearance compiler selects those stacks through variables, including before first paint.
The npm provider reads installed Fontsource packages with remote fetching disabled.
All choices have font declarations, without a second global-face generation path or font preloads.
Fontsource Unicode ranges remain intact; arbitrary chat text is not reduced to UI glyphs.
Operating-system fallbacks cover characters absent from a chosen font.

The image provider resizes only minted IRCord `/files/` paths on the current origin.
It preserves download tokens and leaves external sources, existing queries, blob URLs and data URLs unchanged.
Eligible avatars, direct upload previews and viewer thumbnails request display copies with up to two density candidates.
The backend contains supported still images within a 2048-pixel box without upscaling.
Input limits are 16 MiB and 16 million pixels, with two active transformations.
Its derivative cache holds at most 128 entries or 32 MiB for ten minutes.
Every request rechecks the original file and token before consulting that cache; file responses remain `no-store`.
Animation, orientation metadata, color profiles and unsupported formats retain original bytes.
Larger outputs and busy processing also use original bytes. There is no IPX or arbitrary remote-image proxy.
Animated GIF playback, videos, full-screen originals, offline blobs and downloads retain specialist renderers.

Direct and supported social media can use the configured CDN for 90 days.
The original posted link remains separate from the cached asset URL and its absolute `expiresAt` deadline.
Visible, online previews refresh expired results with bounded retries; background and offline views do not poll.
Successfully rendered direct image/video/GIF URLs are omitted from message presentation only.
Failed, pending and unsupported previews keep their links; captions, code and stored text remain unchanged.
Media controls copy or open the posted link, with hover labels and keyboard/touch access.
The backend retains original-link metadata and cleans expired bytes independently of views.
Storage lifecycle and routing belong to the homelab GitOps configuration.
External/CDN responsive resizing remains outside the owned-upload contract; widget internals retain provider ownership.

VueUse manages scoped key and browser-event listeners, media queries, visibility, resize observation, and selected timeouts.
Keep protocol activity, private WebSockets, appearance persistence, offline storage, and query invalidation with their existing owners.
The early install-prompt listener remains custom because the browser fires that event only once.
The message scroller, app-lifetime update/read-aloud handlers, and shared samplers retain their specialized ownership and disposal.

DevTools uses `enabled: true`; its module runs only during development.
Development also enables A11y. Production does not enable its diagnostics.
The published A11y alpha scans on page load and navigation; constant scanning remains controlled by its panel.
The Fonts developer panel is disabled because its legacy adapter imports H3 v2 APIs into this Nitro 2/H3 v1 app.
Font delivery and adjusted fallbacks remain enabled.
On macOS, Bun defaults to filesystem polling at 250 ms to avoid a stall during native FSEvents registration.
The Chokidar environment settings cover Nuxt, Vite, and DevTools watchers; explicit user settings take precedence.
Other platforms and runtimes keep their default watcher behavior.
ESLint's live checker is disabled; `bun run lint` explicitly generates Nuxt configuration and runs the linter.
Generated contracts, build output, vendored data, and dependencies are excluded.

Localization uses `frontend/i18n/i18n.config.ts` and one English locale assembled from `locales/en.json`
and the `locales/en/{workspace,collections,media}.json` feature catalogs.
Routing uses `no_prefix`, and browser-language detection is disabled. Existing URLs and browser history stay unchanged.
No language selector or second preference store is introduced before additional languages and preference ownership are agreed.
Use stable semantic keys grouped by feature, named values, and Vue I18n plural messages.
Translate interface text only. Preserve messages, nicknames, channel names, protocol commands, and user-authored content.
Shared TypeScript utilities must receive a translator or translated value rather than relying on a component-only composable.
Resolve option labels and shared-helper text when rendered so they follow the active locale.
Preserve numeric plural counts while formatting visible numbers and dates with the active locale.
Strict message compilation stays enabled. Literal `@` and angle-bracket command placeholders need Vue I18n literal interpolation;
one invalid message can prevent the whole locale from loading. Use the installed bundle-utils strict loader for catalog checks.
Open prompts and stored transient notices can retain the language in which they were created.

The initial integration pass ran dependency installation, Nuxt template/type generation, formatting, and static source review.
At the maintainer's request, it ran no lint, typecheck, tests, browser smoke checks, or validation builds.
Later adoption passes cleared the ESLint baseline, checked public metadata privacy and local font assets,
and repaired theme contrast, keyboard access, and status announcements.
The localization integration review checked all English catalogs with Nuxt's strict loader, literal and known dynamic keys,
named interpolation, plural selection, translated timeline/export text, and the complete frontend ESLint pass.
Conversation events use a whole translated sentence with a named actor slot, preserving the profile action and escaped raw text.
These checks do not establish screen-reader conformance, cross-browser coverage, image transfer reductions, or production packaging.
No removed test harness was restored. Publishing a version tag separately runs the release workflows described in `README.md`.

Follow-up startup debugging reproduced the native watcher stall and the Fonts panel's H3 import failure.
With the watcher workaround and Fonts panel disabled, ordinary `bun run dev` reached Ready in 2.52 seconds.
The development server remains available on port 3000. This startup result does not verify application flows or production packaging.

Module references: [Fonts](https://fonts.nuxt.com/), [Image providers](https://image.nuxt.com/get-started/providers),
[VueUse](https://vueuse.org/guide/), [ESLint](https://eslint.nuxt.com/packages/module),
[A11y](https://github.com/nuxt/a11y),
[i18n](https://i18n.nuxtjs.org/).

## Defaults

| Width           | Docked layout                                                    |
| --------------- | ---------------------------------------------------------------- |
| At least 1100px | Conversations, chat, and optional members.                       |
| 900–1099px      | Conversations and chat. Members open in a drawer.                |
| 701–899px       | Chat. Conversations and members open in drawers.                 |
| At most 700px   | Chat fills the width. Conversations and members open in drawers. |

Network tabs and account controls share a 48px top bar at every width.
Search and Inbox live in the conversation list, not the top bar.
The read-aloud switch appears in the top bar only once a speech service is set up.
Tabs scroll horizontally and support Left, Right, Home, and End keys.
Collapse the conversation list from its Search row to keep a 48px icon rail.
Numbered channels follow Favorites, then channel names; unread filters do not renumber them.
Direct messages follow the channels in the rail, without numbers or shortcuts.
Control or Command plus 1–9 selects the matching channel on the current network.
The chat header's search button replaces the member list with a search of that conversation.
Conversation and member widths default to 200px and 208px.
Panel toggles use short, reversible slides from their respective sides.
Initial rendering, responsive layout changes, and drag resizing do not start panel animations.
System and application reduced-motion settings suppress these slides.

Pixel values in this section are at 100% interface scale.
Spacing units, `AppIcon` sizes, `--ui-*` text, header and row heights, panel widths, and dialog frames grow with Appearance → Interface scale.
The timeline and composer reset the scale to 1, so chat follows the message size setting instead.
Prefer spacing units over arbitrary pixel values in interface components, so they scale.
Their resize limits and saved device values belong to `app/stores/layout.ts`.
Conversation headers use the shared 48px height. Touch pointers use a 40px navigation row minimum.
Desktop chat and members share a header and a shadowed frame with rounded top corners.
The frame has an 8px top and side inset and extends to the bottom edge with square corners.
The roster uses the conversation background and a quiet divider inside that frame.
Phone chat uses the full width. Drawers open below the top bar.

Keep the workspace grid row bounded with `minmax(0, 1fr)`.
Scrollable columns must retain `min-height: 0`.
Grouped message bodies stay in their text column when timestamps disappear.
Only a row showing its message toolbar drops rendering containment.

## Failure modes

- Verify component refs after replacing native controls; callers may need DOM identity, selection, focus, or validity methods.
- Verify keyboard focus and composer caret restoration after dialogs, nested menus, and pickers close.
- Verify retained drafts and unsaved forms against update reloads and type-to-compose shortcuts.
- Review selected, unread, muted, disabled, pending, failed, and highlighted states after changing their owners.
- Review both themes, all message layouts, and widths around 700px, 900px, and 1100px.
- Test touch gestures separately from viewport resizing. Emulation does not verify a physical iPhone keyboard or foldable screen.

Use a disposable demo account for browser review, as described in the root README.
Keep screenshots and session files in ignored local storage.
Record actual validation and remaining device checks in the implementation issue.

## Source

- [Frontend development and checks](../README.md#frontend-checks-and-visual-review)
- [Frontend styling](../README.md#frontend-styling)
- [Writing style](STYLE.md)
- [Control and CSS migration](https://github.com/karbowiak/ircord/issues/216)
- [Broader frontend overhaul](https://github.com/karbowiak/ircord/issues/197)
