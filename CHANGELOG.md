# Changelog

What changed for people using IRCord, newest release first. Each release lists what is **New**,
**Improved** and **Fixed**, in plain words. Changes nobody using the app would notice are left out.

## Unreleased

### New

- Hide the network list and the conversation list with the buttons at the top left, and collapse Favorites, Channels or Direct messages by clicking their heading; each device remembers your choice.

### Improved

- Theme, font and message-layout choices in Settings now support arrow-key navigation, with consistent form controls across sections.
- Buttons, fields and error messages now have consistent styling across sign-in, network setup, dialogs, message actions and administration.
- Dialogs, right-click menus, dropdowns and tabs now share consistent keyboard behavior: type a letter to jump to a menu item or option, and use the arrow keys to switch tabs.
- Toolbar buttons in the header, network list, sidebar and composer show their label in a tooltip when you hover over or tab to them.
- Link previews appear at their final size, so the chat no longer jumps as cards, posts and players load; posts from X and Bluesky open in full from a fixed-size preview.
- Notification settings now show at a glance whether notifications work on this device, with one button to turn them on or repair them, and phones and tablets offer an optional setup you can dismiss (including Home Screen help on iPhone and iPad).

### Fixed

- Typing a message keeps the composer's neutral border instead of adding a colored outline.
- Send test in notification settings now notifies only the device you're on, and says so when that device isn't set up for push.
- While you read back through a channel, the new-messages button counts only messages that arrived after you scrolled up, not older history loading above.

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
