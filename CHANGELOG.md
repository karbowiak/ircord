# Changelog

What changed for people using IRCord, newest release first. Each release lists what is **New**,
**Improved** and **Fixed**, in plain words. Changes nobody using the app would notice are left out.

## Unreleased

### Improved

- In the Links view, pick several channels at once, and narrow links to one site or to the days they were posted.
- Busy channels use far less data: a new message no longer resends the whole member list to every device you have open.

### Fixed

- Links in someone's About you text are clickable in their profile and open in a new tab.

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
