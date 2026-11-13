# Changelog

What changed for people using IRCord, newest release first. Each release lists what is **New**,
**Improved** and **Fixed**, in plain words. Changes nobody using the app would notice are left out.

## Unreleased

### Improved

- Deleting your account now asks for your current password.
- Changing your password or deleting your account pauses for a while after repeated wrong passwords.
- Signing in on more than 50 devices signs out the device that signed in longest ago.
- After a working connection drops, IRCord reconnects quickly instead of waiting up to 40 seconds.
- Channels with long histories no longer slow down as new messages arrive.

### Fixed

- Server connection errors appear above the input instead of covering the server status cards.
- Sign-in protection now applies to each visitor separately, so one visitor can no longer affect anyone else signing in or creating an account.
- Slow link previews no longer fail after about 10 seconds.
- An account deletion that fails part way is retried later and no longer stops the server from starting.
- After IRCord restarts, your networks reconnect even when one network can't be restored or a server is briefly unreachable.
- A network that can't be restored after a restart shows why.
- A network keeps your chosen nickname when the server assigns one that IRCord can't use, so it reconnects after a restart.
- Joining is limited to 100 channels per network, so a network with many channels still reconnects after a restart.

## v0.1.29 — 2026-11-11

### Fixed

- Previously shared profile details are republished after reconnecting when their metadata is missing from the IRC server.

## v0.1.28 — 2026-11-11

### New

- Choose from 17 new pixel-art avatars, including retro heads, critters and objects, when you sign up or in your profile settings.

## v0.1.27 — 2026-11-11

### New

- Mark channels as NSFW, reveal individual blurred previews with a click, or enable NSFW content for a server in Profile settings.

### Improved

- Browser Back and Forward retrace pages, dialogs and tabs, including Settings, Administration and media viewers.
- GIF and emoji buttons make room for your message while you type on mobile, then return when you finish editing.

### Fixed

- Larger animated GIFs now preview correctly, including ones previously rejected by the animation size limit.
- Connection interruptions no longer take focus from the message field; you can keep editing your draft while sending reconnects.
- Reopening the app no longer inherits long reconnect waits from retries made while it was in the background.

## v0.1.26 — 2026-11-11

### New

- Create ordinary channels from the Channels menu, with editable templates for private groups, open discussion and announcements.

### Improved

- Group owners can convert existing group conversations to ordinary channels while keeping their messages, access and channel ownership.

### Fixed

- Typing and scrolling send activity reports at most once every ten seconds, while leaving the page promptly releases active status.

## v0.1.25 — 2026-11-10

### Improved

- New accounts can set up their nickname and profile after registration, with completed or skipped setup remembered across devices.

### Fixed

- Adding a server no longer opens first-time profile setup, and server sign-in settings allow saved passwords to be cleared.
- Profile changes stay on the selected server, including display names, pictures, descriptions and name colors.

## v0.1.24 — 2026-11-10

### New

- Start private group conversations, invite people, share earlier messages, and manage participants without configuring an IRC channel.

### Improved

- Find people by nickname, account name or display name, with consistent suggestions and keyboard selection across people pickers.

### Fixed

- Leave group conversations directly from their menu, with confirmation before leaving.
- Starting a direct message uses the selected IRC network, even when names overlap with users on another network.
- Scrolling back to the newest message clears the Jump to present button, and older message windows keep loading toward the present.
- Instagram galleries use smaller photos when originals exceed the preview size limit.
- Reddit posts use their available video preview instead of a still thumbnail.

## v0.1.23 — 2026-11-10

### New

- View database space, configuration, connection usage, and waiting time in Administration.
- Manage signed-in devices and their notification settings from a dedicated Devices page.

### Improved

- Notifications follow the device you engage with, keep the viewed conversation quiet, and group unread catch-up alerts by conversation.
- Choose whether IRCord opens your last viewed conversation or a selected channel. Direct links and notification taps still open their target.
- Public CNE videos, Issuu documents, legacy CARTO maps and Widgetic compositions have dedicated previews with consistent sizes.

### Fixed

- Switching collection filters no longer shows errors from an old request, and empty result pages stop loading at the correct boundary.
- Turning off appearance sync cancels pending saves and keeps newer local changes marked as unsaved.
- Failed network-icon changes release their temporary image data.
- Returning to the app retries interrupted connections automatically. Persistent connection problems appear above the message input without moving the conversation.
- Switching between search and the member list no longer animates the message column through repeated layout changes.
- Gentle upward swipes stay where you leave them instead of snapping back to the newest message.
- Messages you send glide in at the bottom of the chat and stay fully in view, even when a preview or image loads afterwards. The chat no longer jumps around or stops short of your message.
- When you scroll the chat yourself, with the wheel, keys, a swipe or the scrollbar, new messages and loading content no longer pull it back. The chat also stays put while you hold it, and when you send a message while holding it.
- Returning to IRCord on your phone shows the newest messages when you were following the conversation. If you had scrolled up to read, you stay where you were.

## v0.1.22 — 2026-11-10

### Fixed

- Conversation links, settings fields, file sizes and error messages render correctly in release builds.

## v0.1.21 — 2026-11-10

### New

- Drag channels and favorites into your own order, or use Move up and Move down in their menu. New channels join at the bottom, and Sort A to Z restores name order.
- The first nine channels show their number next to the # sign, matching the Ctrl or ⌘ 1–9 shortcuts.

### Improved

- Instagram photos, galleries and reels, and TikTok videos can use native previews from IRCord's media cache.
- The default appearance follows your device's light or dark setting. Saved choices stay unchanged.
- Conversation, member and search panels slide in and out while the chat resizes smoothly, unless reduced motion is selected.
- Supported media links use previews with Copy link and Open link buttons.
- Linked photos, GIFs and videos can load from IRCord's media cache for 90 days, with expired copies fetched again when needed.
- Uploaded images and profile pictures use smaller display copies when their format supports resizing.
- Linked files are checked before previewing, with file details and an original-link action when a preview is unavailable.
- Website links use standard embedded content when available and an information card otherwise.
- More music, podcast, video and image links have dedicated previews with consistent sizes and Copy link and Open link buttons.
- Image previews can grow wider while fitting the chat column, with known dimensions reserved before loading instead of leaving empty space below.
- Choose whether embedded players load automatically or wait for a click in Chat settings.
- Reddit photos and albums appear in compact galleries, with full-screen browsing and thumbnails for every image.
- Gallery embeds share the background color of other link previews across themes.
- Recent link previews survive page reloads and stay visible while their cached details refresh.
- More previews can load together, and scrolling away cancels requests that are no longer needed.
- Link previews reserve their space before loading, with matching frames for media, players, loading states and errors.
- Single photos and GIFs show inline controls, with a small offline button for uploads. GIFs use the available chat width and stay within 200 pixels in height.
- Clicking a photo or GIF opens the full-screen media viewer, while GIFs still play on hover or keyboard focus.
- Drop several images or videos to send one gallery message, with files kept together in history and offline copies.

### Fixed

- Switching conversations releases old message rows instead of retaining their elements and event handlers.
- Browsing image-heavy conversations no longer retains removed message rows through responsive images in Chromium.
- Scrolling through older history now loads messages in both directions without requiring Jump to present.
- Opening message menus avoids updating every loaded message, and timestamps reuse their date formatters.
- Embedded players wait until scrolling pauses before starting.
- Unmuted videos keep playing when the pointer leaves, and native controls no longer fight hover playback.
- Sending a message no longer shows an error from an interrupted refresh.
- Previews show loading feedback while embedded players open instead of leaving a blank box.
- Saving an offline file no longer makes every media preview reload its saved copy.
- Scrolling past embedded players releases them sooner, while preview details can still load ahead of the visible chat.
- Loading older messages and returning to the latest chat no longer skip messages after live updates, reloads or an offline fallback.
- Scrolling through media-heavy chats releases distant players, keeps preview space stable and starts loading older messages sooner.
- The Jump to present button stays clickable above hovered messages.
- Link previews keep their space while messages are being sent or retried.
- Reddit albums with large animations use their smaller video copies instead of losing the entire preview.
- Button labels no longer remain open after closing settings or toggling a sidebar with the mouse.
- Requests from other websites are rejected before they can change settings or saved GIFs.

## v0.1.20 — 2026-11-09

### New

- Read and search the last seven days of saved messages offline, view downloaded photos, and keep files for longer on this device.
- Interface text, help, errors and accessibility labels are ready for translation, starting with English.

### Improved

- Shared links show page-specific previews without including private conversation details.
- Font choices use local font assets and matching fallbacks, including before the app finishes opening.
- Profile pictures, network icons, emoji, link posters and thumbnails have consistent loading and sizing.
- Dates, times, counts and shortcut labels follow the interface language, while keeping your clock and keyboard preferences.
- Check connection status and latency at the bottom of the conversation list, and open server details from the indicator.
- Review message editing support and collapse capabilities, IRCord extensions, and server settings in the Server panel.
- Restore your workspace from this device, catch up on missed changes, and keep recent messages current when reconnecting.
- Switch networks from tabs across the top, with profile and settings controls beside them, a compact conversation list, and a clearer chat panel.
- Collapse the conversation list into an icon rail with numbered channels and your direct messages. Use Ctrl or Command plus 1–9 to switch channels on the current network.
- Open Search, Inbox, Saved, Drafts and Links as pages with clearer previews, simpler filters, and browser history support.
- Scale text, icons, spacing and panels in Appearance without changing message text; message settings are now grouped under Chat.
- Narrow message search by sender, mention, channel, content type, or date. Pick filters from suggestions or the Filters dialog. Recent searches stay on this device.
- Search stored history with filters alone, and load older matching messages from Search or a conversation's search panel.
- Search the current conversation from its header, with results beside the chat. Use the same filters or widen the search to everywhere.
- Browse link previews, open photos and videos full screen, play videos inline, and choose channels and content types from searchable menus. GIFs play while you hover over them.
- Review connection status, latency for each hop, TLS certificate details and security grades, and server capabilities beside the log.
- Read-aloud controls appear in the top bar after you set up a speech service.
- Zoom photos with labeled controls in the full-screen viewer.
- Text, links and control boundaries are easier to read across appearance schemes, including muted conversations and ignored names.
- Reach conversation, network and message actions from the keyboard, with checked menu choices, clearer unavailable-action explanations and named unread, mention and presence states.
- Screen readers hear one throttled update for new messages, without repeating loaded history, and can find conversation headings and date boundaries.
- Notices stay visible longer and pause while hovered or focused; error notices can stay until dismissed.

### Fixed

- Live connection logs now stop when a session expires or is revoked, and keep the device's last-seen time current.
- Mobile, foldable, and tablet layouts keep inputs aligned, page controls visible, and server details accessible.
- Workspace pages stay open after a reload when the previously selected conversation has closed.
- Sign-in and sign-up explain invalid fields and focus the first one, including blank usernames and unavailable names.
- Reaching the message box from the keyboard shows a neutral focus indicator; clicking in and typing keep its plain appearance.

## v0.1.19 — 2026-11-09

### New

- Settings › About now lists the people who build IRCord.
- Type /afk or /brb, with an optional reason, to show everyone on IRC that you are away from your keyboard or right back. Your next message marks you back, as does /back. Your away message now also shows under your name and on profiles.

### Fixed

- Logging out on your last signed-in device now takes you off IRC, and you stay off until you sign in again, even when IRCord restarts. While another device is still signed in, you stay connected.
- IRCord no longer rejoins a channel you were kicked from when it reconnects. The channel shows who kicked you and why, with a Rejoin button.
- Your name no longer disappears from your new messages and the member list after the app opens its offline copy. This happened when you reloaded while IRCord was restarting, and only another reload brought your name back.

## v0.1.18 — 2026-11-09

### New

- Change your keyboard shortcuts in Settings › Shortcuts: give an action other keys, remove keys or reset to the defaults. New shortcuts move between networks, jump to the first unread or newest message, and focus the message box. Your shortcuts follow your account.
- Add bans, exceptions, invite exceptions and quiets by choosing someone from the channel and checking what to match them by: nick, username, host or, where the network supports it, their account. The mask itself is still there to read or edit under Advanced.

### Improved

- Theme, font and message-layout choices in Settings now support arrow-key navigation, with consistent form controls across sections.
- Buttons, fields and error messages now have consistent styling across sign-in, network setup, dialogs, message actions and administration.
- Dialogs, right-click menus, dropdowns and tabs now share consistent keyboard behavior: type a letter to jump to a menu item or option, and use the arrow keys to switch tabs.
- Toolbar buttons in the header, network list, sidebar and composer show their label in a tooltip when you hover over or tab to them.
- Your nick counts as a mention only with an @ in front, such as @alice, so messages that only name you no longer highlight or notify you. To be notified when people type your nick without the @, add it to your highlight words. Tab completion now adds the @.

### Fixed

- The toolbar that appears when you hover over a message is no longer partly hidden behind the message or image above it.
- Warnings in network setup and sign-in, such as a nickname the network will refuse, now show in red.
- On phones and small screens, the sign-in and sign-up forms scroll, and the field you're typing in stays visible above the keyboard, there and in dialogs.
- Typing a message keeps the composer's neutral border instead of adding a colored outline.
- Pinching a photo in the full-screen viewer now zooms the photo instead of the whole app. Drag to move around a zoomed photo, and double-tap or double-click to zoom in or back out. On a computer, a trackpad pinch, Ctrl and the scroll wheel, or the + and - keys also zoom, and 0 resets.

## v0.1.17 — 2026-11-08

### New

- Use `@here` or `@channel` (everyone in the channel who isn't away), `@everyone`, or `@owner`, `@operator` and `@voiced` to notify a group at once; they show as mention pills and are offered when you type `@`. Channel operators choose in Channel settings who may use `@here`, `@channel` and `@everyone`: anyone, operators only (the default) or nobody.

## v0.1.16 — 2026-11-08

### New

- Pick a profile picture from 32 anime-style avatars when you sign up or in your profile settings, or upload your own; sign-up suggests one based on your username.

## v0.1.15 — 2026-11-08

### New

- Have messages that mention you read aloud: set up an OpenAI-compatible text-to-speech service under Settings → Read aloud, then switch it on with the speaker button beside Settings.
- Hide the network list and the conversation list with the buttons at the top left, and collapse Favorites, Channels or Direct messages by clicking their heading; each device remembers your choice.
- Start a line with `> ` to show it as a quote, or `>>> ` to quote the rest of your message, as in Discord.

### Improved

- Link previews appear at their final size, so the chat no longer jumps as cards, posts and players load; posts from X and Bluesky open in full from a fixed-size preview.
- Notification settings now show at a glance whether notifications work on this device, with one button to turn them on or repair them, and phones and tablets offer an optional setup you can dismiss (including Home Screen help on iPhone and iPad).

### Fixed

- Send test in notification settings now notifies only the device you're on, and says so when that device isn't set up for push.
- While you read back through a channel, the new-messages button counts only messages that arrived after you scrolled up, not older history loading above.
- In most browsers, moving the pointer off a playing GIF now pauses it on the frame it reached, and it carries on from there next time, for GIFs from the picker and from Giphy, Tenor and Imgur.

## v0.1.14 — 2026-11-08

### New

- Slap someone with `/slap nick`: everyone sees you slap them around a bit with a large cucumber.
- Reddit, Instagram and TikTok posts can show photos, videos and albums directly in chat, with embedded widgets for other sites and when direct media is unavailable.
- Server owners can view service status and manage server ownership in the new Administration panel.
- Type /ignore with a nickname to hide someone's messages, /unignore to bring them back, or /ignore alone to see who you're ignoring.

### Improved

- In the Links view, pick several channels at once, and narrow links to one site or to the days they were posted.
- Busy channels use far less data: a new message no longer resends the whole member list to every device you have open.
- Reddit, Instagram and TikTok cards take the shape of their photo or video, show who posted them, and let you swipe through albums; photos open in a full-screen viewer.
- TikTok videos play right in the chat instead of in TikTok's own player.
- Direct image links and uploads show labeled previews; GIFs and videos wait for hover or explicit playback, with downloads available.

### Fixed

- Links in someone's About you text are clickable in their profile and open in a new tab.
- Scrolling through a channel full of links and embeds no longer jumps around while previews load.
- Ordinary web pages can show link cards, and unavailable or restricted posts explain why their previews could not load.
- Link cards keep small thumbnails beside the text and avoid repeating the original link beneath the embed.
- GitHub cards show repository and issue context, and embedded players respect the height requested by their provider.
- YouTube embeds receive the site identification they require instead of failing with player configuration error 153.
- Paging through a gallery keeps its height steady, including when one item cannot load; other items remain available.
- Responsive embeds keep the layout supplied by their provider instead of leaving empty space beneath videos.
- CNN video players load when clicked, keeping their advertising scripts idle while you read the conversation.
- Resolved links are reused for up to seven days, refreshed sooner when signed media expires; loading previews preserve your reading position and retries show their progress.

## v0.1.13 — 2026-11-08

### New

- Find every link posted in a channel, or across all your channels, in the new Links view, and jump straight to the message that shared it.
- Lists show as lists: start lines with `-`, `*` or `1.` (indent to nest), or paste a list copied from a web page and it arrives ready to send.
- When you step away, only the device you used last is notified at first, and every device a few minutes later with one reminder of what's still unread. Set both delays in Settings → Notifications.
- Other people on IRC see you as away after a while with no IRCord device connected, and back as soon as you return. You can turn this off in Settings → Notifications.
- People using IRCord on a phone or tablet show that device in place of their presence dot.
- Edit your own messages: use the pencil or the message menu, or press Up in an empty message box. Edited messages show “(edited)” with their earlier versions, and people on other IRC apps see your correction as a new line.

## v0.1.12 — 2026-11-08

### New

- Browse every image and GIF posted in a channel in its new gallery.
- Channels you create on the IRCord network are registered to your account automatically. You can turn this off in Settings → Networks.
- Channel registration forms suggest the accounts of people in the channel.
- On servers with media hosting turned on, images and videos linked in messages load from IRCord's own copy, so the site they came from never sees you. They show a loading state while the copy is prepared, and an explanation if it was removed or can't be loaded.
- Ban and exception lists explain what each entry does, show the exact mask that will be added and warn about masks that would match too many people.

### Improved

- Deleted messages now disappear from the timeline; replies that quoted them still show the quote as deleted.
- The time of joins, parts and other events now sits before the event instead of at the far right.
- Channels and favorites are sorted by name in the sidebar.
- The user panel, message box and a new member search share one look, and search now sits above the member list.
- Connection dots next to a network only appear when something is wrong.
- "Who's typing" now shows just above the message box, which sits closer to the bottom of the window.
- Deleting your IRCord account now retires its username for good and also removes its account on the network.

### Fixed

- Pressing Enter sends your message when a mention or channel name is already typed out, instead of completing it again.
- The mention list closes once a completed name is followed by a space.
- The timeline keeps following the conversation when the keyboard opens or the message box grows.
- Reconnecting no longer posts a fresh "joined" notice for every channel you never left.
- Your name and avatar no longer get wiped or replaced when another IRCord server signs in to the same network account.
