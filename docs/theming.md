# Personal themes

This reference defines what a personal theme can change, what IRCord promises to keep stable for
theme authors, and how IRCord keeps the app usable when a theme breaks it.

## Contract

A theme is a named document in the account's theme library. It has three optional parts:

- **Colors.** The six seed colors and eight palette hues of a built-in scheme, for dark mode,
  light mode or both. A mode without its own colors uses the base scheme unchanged. IRCord
  derives every interface token from the seeds and adjusts text colors to 4.5:1 contrast.
- **Background.** One image uploaded to the account, behind the conversation or the whole app,
  with fit, position, dim, blur and panel opacity. IRCord sets a lower limit on dim and panel
  opacity from the image's measured tones, so text keeps its contrast.
- **Custom CSS.** Up to 64 KB (counted in UTF-8 bytes) and 1,000 rules, keyframe frames
  included. It applies to the workspace only.

### Stable variables

Themes that use these variables should keep working after app updates.

| Variable | Meaning |
| --- | --- |
| `--rail`, `--sidebar`, `--main`, `--panel`, `--raised`, `--hover`, `--input` | Surface colors, darkest to lightest in dark mode |
| `--border`, `--control-border` | Dividers, and field and checkbox edges at 3:1 |
| `--text`, `--body`, `--muted`, `--faint` | Text colors, strongest to quietest |
| `--accent`, `--accent-ink`, `--selection` | Accent, text on the accent, and the selected row |
| `--mint`, `--danger`, `--yellow` | Status colors |
| `--code-keyword`, `--code-string`, `--code-number`, `--code-comment`, `--code-function`, `--code-type`, `--code-property`, `--code-meta`, `--code-deletion` | Syntax colors in code blocks |
| `--font-ui`, `--font-message`, `--font-mono` | Interface, message and code typefaces |
| `--message-size`, `--group-gap` | Message text size and the space between message groups |
| `--radius`, `--radius-large` | Corner radii |
| `--interface-scale` | Interface scale as a number, such as `1` for 100% |
| `--theme-image` | The theme's background image, when it has one |

When custom CSS sets one of these variables on `:root` or `html`, Settings → Appearance marks
the matching setting "Set by your theme's CSS".

### Stable root attributes

These attributes are on `<html>`:

| Attribute | Values |
| --- | --- |
| `data-theme` | `dark` or `light` |
| `data-scheme` | The base scheme's family, such as `ircord` or `hibiki` |
| `data-layout` | `cozy`, `compact` or `irc` |
| `data-density` | `comfortable` or `compact` |
| `data-corners` | `rounded`, `soft` or `square` |
| `data-motion` | `full` or `reduced` |
| `data-user-theme` | Present while a personal theme is in use |
| `data-theme-background` | `conversation` or `app`, while the theme draws a background |

A theme based on Hibiki keeps `data-scheme="hibiki"`, so Hibiki's glow and selected-row style
stay without copying its CSS.

### Stable class names

These class names for the shell and the timeline stay stable. A change to one of them gets a
line in the changelog.

| Class | Element |
| --- | --- |
| `.workspace` | The whole signed-in app |
| `.workspace-bar` | The top bar with the network tabs |
| `.conversation-sidebar` | The channel and conversation list |
| `.conversation-link` | One row in that list; `.is-selected` marks the open one |
| `.chat-main` | The framed area that holds the open conversation or page |
| `.chat-header` | The conversation's title bar |
| `.member-sidebar` | The member list |
| `.message-row` | One message; `.is-mention` marks a mention of you |
| `.author-name` | The sender's name on a message |
| `.message-text` | The text of a message |
| `.composer-area` | The message box and what sits above it |

Other class names work too, but they can change in any release.

### What custom CSS cannot do

The filter parses the CSS with the browser's own parser and keeps only what it serializes again.
It removes:

- Anything that loads a file: `url()`, `image()`, `image-set()`, `cross-fade()`, `element()`,
  `paint()` and `src()` in any value, and `@import`. Use the theme background for images.
- `@font-face`. Name a font that is installed on the device instead.
- Backslash escapes where the browser keeps the raw text: custom properties and values that use
  `var()`, `env()` or `attr()`.
- Rule types other than style rules, `@media`, `@supports`, `@container`, `@keyframes`,
  `@property` and `@starting-style`.
- Cascade layers. The rules inside an `@layer` block stay and apply without the layer, and
  `@layer` statements are removed. Theme CSS is always unlayered, so it can't join or reorder
  the app's own layers.

The studio and the import summary list each removal.

## Defaults

- Each account can keep 20 themes. The library is saved to the account whatever the appearance
  sync switch says. The theme in use is part of Appearance and follows that switch.
- Custom CSS is on for each account and on for each server. The account switch is under
  Settings → Appearance → Your themes. Operators turn it off for the server with
  `IRCORD_CUSTOM_CSS=false`. With either switch off, themes keep their CSS, and colors and
  backgrounds still apply.
- Export writes a `.ircord-theme.json` file with colors, background settings and CSS. It never
  includes the image, because an upload address is a download link.
- An imported theme is added to the library and is not applied.

## Failure modes

Each of these recovers from a broken theme without editing it:

- **The countdown.** Applying a theme with CSS, or saving new CSS for the theme in use, shows
  "Keep this theme?". Without Keep, the theme reverts after 20 seconds. Only Keep saves the
  choice, so a reload during the countdown also returns to the previous theme or CSS.
- **Protected screens.** Custom CSS is off while sign-in, Settings, Administration, the welcome
  and network forms, or a confirmation is open. Colors and backgrounds still reach Settings.
- **The shortcut.** Shift F8 pauses the theme for this visit. Press it again to resume.
- **Safe mode.** Open the app with `?safe` in the address, such as
  `https://chat.example/?safe`. The theme stays off for that visit, from before the first paint.
- **Another device.** With sync on, choosing "Use built-in scheme" on another device turns the
  theme off here too.

The keep-or-revert bar, the safe-mode banner and the theme studio render in a shadow root in the
browser's top layer. Theme selectors cannot match them, and nothing can cover them. Two app
rules hold whatever a theme says, because they use `!important` inside the app's cascade layers
and theme CSS is never layered: `html` and `body` stay displayed, and reduced motion wins over
theme animations.

A theme saved by a newer IRCord is shown as "Made for a newer IRCord" and is not applied.

## Source

- `frontend/shared/themes.ts`: the theme document, limits, import, export and background floors
- `frontend/shared/theme-css.ts`: the custom CSS filter
- `frontend/app/stores/themes.ts`: the library, the studio draft and the recovery paths
- `frontend/app/components/settings/themes/`: the library, the studio and the isolated layer
- [Frontend components and styling](frontend.md)
- [Issue #88](https://github.com/karbowiak/ircord/issues/88)
