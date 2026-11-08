# Changelog

What changed for people using IRCord, newest release first. Each release lists what is **New**,
**Improved** and **Fixed**, in plain words. Changes nobody using the app would notice are left out.

## Unreleased

### New

- Find every link posted in a channel, or across all your channels, in the new Links view, and jump straight to the message that shared it.
- Lists show as lists: start lines with `-`, `*` or `1.` (indent to nest), or paste a list copied from a web page and it arrives ready to send.
- Reddit, Instagram and TikTok posts can show photos, videos and albums directly in chat, with embedded widgets for other sites and when direct media is unavailable.

### Improved

- Reddit, Instagram and TikTok cards take the shape of their photo or video, show who posted them, and let you swipe through albums; photos open in a full-screen viewer.
- TikTok videos play right in the chat instead of in TikTok's own player.

### Fixed

- Scrolling through a channel full of links and embeds no longer jumps around while previews load.
- Ordinary web pages can show link cards, and unavailable or restricted posts explain why their previews could not load.
- Link cards keep small thumbnails beside the text and avoid repeating the original link beneath the embed.
- GitHub cards show repository and issue context, and embedded players respect the height requested by their provider.
- YouTube embeds receive the site identification they require instead of failing with player configuration error 153.
- Paging through a gallery keeps its height steady, including when one item cannot load; other items remain available.
- Responsive embeds keep the layout supplied by their provider instead of leaving empty space beneath videos.
- CNN video players load when clicked, keeping their advertising scripts idle while you read the conversation.
- Resolved links are reused for up to seven days, refreshed sooner when signed media expires; loading previews preserve your reading position and retries show their progress.

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
