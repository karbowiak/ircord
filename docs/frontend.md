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
Tailwind Preflight remains omitted; `base.css` supplies normalization and focus behavior.

### Component inventory

| Surface                      | Owners and disposition                                                                                                                                          |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shared controls              | `UiButton`, `UiInput`, `UiTextarea`, `UiLabel`, `UiCheckbox`, `UiSwitch`, `UiRadioGroup`, `UiSelect`, and `UiSlider` use the shared theme.                      |
| Labels and validation        | `AppField` associates labels, help, and errors with controls. `AppInput` forwards native attributes and exposes focus. Rich hints use slots.                    |
| Sign-in and registration     | `LoginScreen` uses shared fields. Its scroll area and `keep-field-in-view.client.ts` handle short screens and visual viewport changes.                          |
| Loading and notices          | `BootScreen`, `EmptyState`, `UiAlert`, `UiBadge`, and `ConnectionBanners` own their presentation.                                                               |
| Settings navigation          | `SettingsLayout` and `SettingsNavItem` serve Settings, Administration, and Channel settings. Section navigation retains `aria-current`; it is not a tablist.    |
| Settings content             | Shared titles, descriptions, groups, rows, notes, and links provide common presentation. `ChipInput` owns editable word and nickname lists.                     |
| Network setup                | `NetworkDialog` composes Server, Identity, Login, and On connect fields. `useNetworkForm` owns shared form state.                                               |
| Channel settings             | `ChannelOverview`, `ChannelModes`, `ChannelModeList`, and `ChannelRegistration` retain separate responsibilities. `useChannelContext` supplies their context.   |
| Notifications                | `NotificationDevice` and `NotificationRouting` split device setup from notification routing.                                                                    |
| Dialogs and media viewer     | `AppDialog` and `MediaViewer` use Reka Dialog. `useDialogStack` coordinates dismissal and update/typing guards.                                                 |
| Menus, selects, and tooltips | Context menus use Reka DropdownMenu. Desktop selects use Reka Select; touch selects remain native. `AppTooltip` shares the app-wide provider.                   |
| Result views                 | `SearchField`, `ConversationResult`, and `MessageResult` serve Search, Saved, Inbox, and Links. Search, Inbox, and server-detail tabs use Reka Tabs.            |
| Links                        | `LinkChannelPicker` owns the inline channel disclosure. `LinkCatalogCard` owns each resource and its source-message rows.                                       |
| Navigation                   | `NetworkRail`, `ConversationSidebar`, `ConversationLink`, `ChatHeader`, and `MemberSidebar` own ordinary presentation. Docking follows `workspace-layout.css`.  |
| Timeline                     | `ChatTimeline`, `MessageItem`, `EventRow`, `MessageExtras`, and attachment components own their markup. `timeline.css` coordinates layout modes and row states. |
| Composer                     | `ChatComposer`, `ComposerSuggestions`, and `ComposerUploads` retain native editing, completion, and upload behavior with component utilities.                   |
| Server window                | `ServerWindow` and `ServerConnectionDetails` own log, handshake, filter, and table presentation.                                                                |
| Identity and media           | Existing avatar, identity-color, attachment, and link-media components retain their loading, permission, and playback behavior.                                 |

### Retained custom controls

- `AccountCombobox` allows free text. Enter submits that text unless a suggestion is highlighted.
- `AdminAccountPicker` displays results inline rather than in a popup.
- The expression picker supports resizing, viewport clamping, and a phone sheet layout.
- Composer completion preserves the textarea caret, input method composition, and native editing.
- Message action buttons retain native titles, including explanations for disabled actions.
- File and color inputs retain their native controls.
- Navigation rows, avatar controls, resize handles, and game controls retain their domain-specific native markup.

### Stylesheet ownership

| File                                        | Purpose                                                                                                   |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `fonts.css`, `base.css`, `theme.css`        | Fonts, normalization, global geometry, focus, and semantic utility tokens.                                |
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

## Defaults

| Width           | Docked layout                                             |
| --------------- | --------------------------------------------------------- |
| At least 1100px | Rail, conversations, chat, optional members.              |
| 900–1099px      | Rail, conversations, chat. Members open in a drawer.      |
| 701–899px       | Rail and chat. Conversations and members open in drawers. |
| At most 700px   | Chat. Navigation opens in drawers.                        |

The rail is 64px wide. Conversation and member widths default to 248px and 232px.
Their resize limits and saved device values belong to `app/stores/layout.ts`.
Headers use the shared 48px height. Touch pointers use a 40px navigation row minimum.

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
