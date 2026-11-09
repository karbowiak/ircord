# Frontend components and styling

This reference describes the frontend component boundaries and retained specialist styling.
Use it when changing shared controls, workspace layouts, or appearance behavior.

## Contract

`frontend/app/assets/css/tailwind.css` is the single stylesheet entry point.
Its layers are `theme`, `base`, `components`, `utilities`, and `specialist`, in that order.
The legacy layer and its control, dialog, and workspace files are removed.

`frontend/shared/appearance.ts` owns appearance values and semantic tokens.
The appearance store and pre-paint script apply saved values.
Keep this authority when generating or updating UI primitives.
Interface scale defaults to 100% with a 14px base and ranges from 85% to 150%.
Use the shared `--ui-*` type variables for navigation and shared controls.
Message text keeps its separate size under Chat. Interface scale follows the existing appearance sync and reset behavior.
Tailwind Preflight remains omitted; `base.css` supplies normalization and focus behavior.

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
`app/app.vue` imports the single workspace layout directly. Authentication gates the workspace in `app/layouts/default.vue`.
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

| Integration                                 | Ownership                                                                                    |
| ------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Built-in head metadata and `@nuxtjs/robots` | Generic incoming link cards, crawl refusal, and indexing headers.                            |
| `@nuxt/fonts`                               | Fontsource assets, Unicode ranges, and adjusted local fallback metrics.                      |
| `@nuxt/image`                               | Avatar, link-card, and gallery-thumbnail presentation through the `ircord` provider.         |
| `@vueuse/nuxt`                              | Auto-imported lifecycle helpers; the existing stores and query cache retain their ownership. |
| `@nuxt/eslint`                              | Nuxt-aware Vue and TypeScript correctness rules. Prettier owns formatting.                   |
| `@nuxt/hints`                               | Development-only Web Vitals and third-party script diagnostics.                              |
| `@nuxt/a11y`                                | Opt-in development accessibility feedback.                                                   |
| `@nuxtjs/i18n`                              | English-only localization infrastructure. String extraction is deferred.                     |

The app remains client rendered. Public metadata uses only the generic description and existing static logo.
The Nitro `render:html` hook adds absolute image URLs before the response reaches preview clients.
Set `NUXT_SITE_URL` to the public origin behind a reverse proxy.
Without that setting, the frontend uses the forwarded request origin; the proxy must replace untrusted forwarding headers.
No conversation path, query, channel, message, account, or member data enters these tags.
All workspace pages use `noindex, nofollow`, and `robots.txt` disallows crawling.
These directives do not provide access control. There is no sitemap or chat-derived structured data.

Font family names and saved appearance keys stay stable.
`fonts.css` exposes literal family stacks so Nuxt Fonts can add metric-adjusted fallbacks.
The appearance compiler selects those stacks through variables, including before first paint.
The npm provider reads installed Fontsource packages with remote fetching disabled.
All choices have font declarations, but unused faces are not preloaded.
Fontsource Unicode ranges remain intact; arbitrary chat text is not reduced to UI glyphs.
Operating-system fallbacks cover characters absent from a chosen font.

The image provider passes backend-issued URLs through without changing tokens, query strings, caching, or access rules.
The browser loads those URLs directly; there is no IPX proxy or new native image runtime.
Dimensions and loading behavior belong to the consuming component.
Animated GIF playback, videos, full-screen originals, and original downloads retain their specialist renderers.
The CDN currently serves originals and has no resize endpoint. Responsive derivatives and transfer-size reductions remain deferred.
Before adding variants, define bounded dimensions, authorization, cache keys, retention, and deletion behavior in the backend/CDN.
Do not add width parameters or density candidates that claim nonexistent derivatives.

VueUse currently manages workspace key listeners, shared Shift-key state, and media-caption resize observation.
Keep protocol activity, private WebSockets, appearance persistence, offline storage, and query invalidation with their existing owners.
The early install-prompt listener remains custom because the browser fires that event only once.

Development enables Nuxt DevTools and Hints. Production enables neither diagnostics module.
Hints records Web Vitals and script use; SSR validation, hydration checks, and lazy-component suggestions are disabled for this client-rendered workspace.
To enable A11y, set `IRCORD_DEV_A11Y=true` before starting the development server.
The published alpha scans on page load and navigation when enabled; constant scanning remains controlled by its panel.
It is off by default to avoid unsolicited accessibility runs during normal development.
ESLint's live checker is disabled; `bun run lint` explicitly generates Nuxt configuration and runs the linter.
Generated contracts, build output, vendored data, and dependencies are excluded.

Localization uses `frontend/i18n/locales/en.json` and `frontend/i18n/i18n.config.ts`.
The English catalog is intentionally empty until extraction begins.
Routing uses `no_prefix`, and browser-language detection is disabled. Existing URLs and browser history stay unchanged.
No language selector or second preference store is introduced before additional languages and preference ownership are agreed.
For later extraction, use stable semantic keys grouped by feature, parameterized values, and plural messages.
Translate interface text only. Preserve messages, nicknames, channel names, protocol commands, and user-authored content.
Shared TypeScript utilities must receive a translator or translated value rather than relying on a component-only composable.
Extract labels, validation, notifications, accessibility text, and date/number formatting as a separate coherent pass.

This integration pass ran dependency installation, Nuxt template/type generation, formatting, and static source review.
At the maintainer's request, it ran no lint, typecheck, tests, browser smoke checks, or validation builds.
Runtime behavior, the initial lint baseline, font metrics, transfer measurements, and production packaging remain unverified in this pass.
No removed test harness was restored, and no release or deployment is included.

Module references: [Fonts](https://fonts.nuxt.com/), [Image providers](https://image.nuxt.com/get-started/providers),
[VueUse](https://vueuse.org/guide/), [ESLint](https://eslint.nuxt.com/packages/module),
[Hints](https://github.com/nuxt/hints), [A11y](https://github.com/nuxt/a11y),
[i18n](https://i18n.nuxtjs.org/), and [Robots](https://nuxtseo.com/robots).

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
