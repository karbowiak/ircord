import type { Bootstrap, Message, Network } from '../../shared/types/chat'
import { wireConversation } from '../../shared/utils/conversation'

/** Connection details and ISUPPORT every demo network shares. */
function networkDefaults(
  name: string,
  extra: Partial<Pick<Network, 'auth' | 'autoJoin' | 'features'>> = {},
): Pick<
  Network,
  'nick' | 'username' | 'realname' | 'auth' | 'autoJoin' | 'perform' | 'isupport' | 'features'
> {
  return {
    nick: 'karbowiak',
    username: 'karbowiak',
    realname: 'Karbowiak',
    auth: { method: 'none', passwordSet: false },
    autoJoin: [],
    perform: [],
    isupport: {
      NETWORK: name,
      PREFIX: '(qaohv)~&@%+',
      CHANMODES: 'beIq,k,fjl,CPQSTcgimnprstuz',
      CHANTYPES: '#',
      CASEMAPPING: 'rfc1459',
      NICKLEN: '31',
      TOPICLEN: '390',
    },
    features: {
      setname: true,
      metadata: [],
      away: true,
      redact: true,
      replies: true,
      reactions: true,
      typing: true,
      attract: true,
      readMarkers: true,
      history: true,
      messageLineBytes: 310,
      multiline: { maxBytes: 4096, maxLines: 100 },
    },
    ...extra,
  }
}

export function createFixtures(): { bootstrap: Bootstrap; messages: Record<string, Message[]> } {
  const members: Bootstrap['members'] = [
    {
      id: 'you',
      nick: 'karbowiak',
      displayName: 'Karbowiak',
      nameSource: 'profile',
      color: 'lavender',
      status: 'online',
      role: 'member',
      bio: 'Making things for the internet. Usually with a coffee nearby.',
      location: 'Copenhagen, Denmark',
      account: 'karbowiak',
    },
    {
      id: 'mika',
      avatarUrl: '/avatars/mika.jpg',
      nick: 'mika',
      displayName: 'Mika',
      color: 'mint',
      status: 'online',
      role: 'operator',
      bio: 'Keeping the lights on. Open source, good coffee, long walks.',
      location: 'Helsinki, Finland',
      account: 'mika',
    },
    {
      id: 'jules',
      avatarUrl: '/avatars/jules.jpg',
      nick: 'jules',
      displayName: 'Jules',
      color: 'peach',
      status: 'online',
      role: 'member',
      bio: 'Designing little corners of the web. Probably rearranging my desk.',
      location: 'Amsterdam, Netherlands',
      account: 'jules',
    },
    {
      id: 'noah',
      avatarUrl: '/avatars/noah.jpg',
      nick: 'noah',
      displayName: 'Noah',
      color: 'blue',
      status: 'online',
      role: 'member',
      bio: 'Photos, mountains, and the occasional side project.',
      location: 'Bergen, Norway',
      account: 'noah',
    },
    {
      id: 'sage',
      avatarUrl: '/avatars/sage.jpg',
      nick: 'sage',
      displayName: 'Sage',
      color: 'rose',
      status: 'online',
      role: 'member',
      bio: 'A little bit of everything. Here for the conversation.',
      account: 'sage',
    },
    {
      id: 'ada',
      nick: 'ada',
      displayName: 'Ada',
      color: 'yellow',
      status: 'away',
      role: 'member',
      bio: 'Out finding something interesting. Back soon.',
      account: 'ada',
    },
    {
      id: 'lin',
      nick: 'lin',
      displayName: 'Lin',
      color: 'lavender',
      status: 'away',
      role: 'member',
      bio: 'Music on, notifications off.',
      account: 'lin',
    },
    {
      id: 'ellis',
      nick: 'ellis',
      displayName: 'Ellis',
      color: 'blue',
      status: 'offline',
      role: 'member',
      bio: 'See you around.',
      account: 'ellis',
    },
    {
      id: 'echo',
      nick: 'echo',
      displayName: 'Echo',
      color: 'mint',
      status: 'online',
      role: 'bot',
      bio: 'Your friendly echo. Send a message in any conversation and I’ll send it right back. This is a demo; no IRC network is connected.',
      account: 'echo',
    },
  ]
  const bootstrap: Bootstrap = {
    version: 1,
    invitations: [],
    demo: true,
    capabilities: {
      serverPreviews: true,
      upload: {
        maxBytes: 8 * 1024 * 1024,
        types: ['image/*', 'video/mp4', 'text/plain', 'application/pdf'],
      },
    },
    user: members[0]!,
    members,
    networks: [
      {
        id: 'libera',
        name: 'Libera Chat',
        host: 'irc.libera.chat',
        port: 6697,
        tls: true,
        initials: 'L',
        color: 'mint',
        status: 'connected',
        capabilities: ['replies', 'reactions', 'history', 'profiles'],
        ...networkDefaults('Libera.Chat', {
          auth: { method: 'sasl-plain', account: 'karbowiak', passwordSet: true },
          autoJoin: ['#general', '#development'],
        }),
      },
      {
        id: 'ergo',
        name: 'Ergo',
        host: 'irc.example.test',
        port: 6697,
        tls: true,
        initials: 'e',
        color: 'lavender',
        status: 'connected',
        capabilities: ['replies', 'reactions', 'history', 'profiles'],
        ...networkDefaults('ErgoTest'),
      },
      {
        id: 'oftc',
        name: 'OFTC',
        host: 'irc.oftc.net',
        port: 6697,
        tls: true,
        initials: 'O',
        color: 'peach',
        status: 'connected',
        capabilities: ['history'],
        ...networkDefaults('OFTC', {
          features: {
            setname: false,
            metadata: [],
            away: true,
            redact: false,
            replies: false,
            reactions: false,
            typing: false,
            attract: false,
            readMarkers: false,
            history: false,
            messageLineBytes: 310,
          },
        }),
      },
    ],
    conversations: [
      {
        id: 'general',
        networkId: 'libera',
        kind: 'channel',
        name: 'general',
        topic: 'Good people, good conversations. Make yourself at home.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: true,
        memberIds: members.map((m) => m.id),
        memberModes: { mika: '@', you: '@', jules: '+', echo: '+' },
        modes: 'nt',
        topicSetBy: 'mika',
        topicSetAt: '2026-09-14T08:12:00.000Z',
        createdAt: '2024-03-02T19:45:00.000Z',
        url: 'https://ircord.example/general',
      },
      {
        id: 'introductions',
        networkId: 'libera',
        kind: 'channel',
        name: 'introductions',
        topic: 'New around here? Say hello. We’re glad you found us.',
        unreadCount: 3,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'mika', 'jules', 'sage', 'echo'],
      },
      {
        id: 'development',
        networkId: 'libera',
        kind: 'channel',
        name: 'development',
        topic: 'Things we’re building, breaking, and figuring out together.',
        unreadCount: 8,
        mentionCount: 2,
        favorite: true,
        memberIds: ['you', 'mika', 'noah', 'lin', 'echo'],
      },
      {
        id: 'design',
        networkId: 'libera',
        kind: 'channel',
        name: 'design',
        topic: 'A place for the details. Share something that caught your eye.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'jules', 'sage', 'echo'],
      },
      {
        id: 'off-topic',
        networkId: 'libera',
        kind: 'channel',
        name: 'off-topic',
        topic: 'Everything else belongs here.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'mika', 'ada', 'noah', 'echo'],
      },
      {
        id: 'music',
        networkId: 'libera',
        kind: 'channel',
        name: 'music',
        topic: 'What’s in your headphones?',
        unreadCount: 2,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'lin', 'sage', 'echo'],
      },
      {
        id: 'ergo-lounge',
        networkId: 'ergo',
        kind: 'channel',
        name: 'lounge',
        topic: 'The beginning of something good.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'mika', 'echo'],
      },
      {
        id: 'oftc-general',
        networkId: 'oftc',
        kind: 'channel',
        name: 'community',
        topic: 'An open place to meet.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'ada', 'echo'],
      },
      {
        id: 'dm-jules',
        networkId: 'libera',
        kind: 'direct',
        name: 'jules',
        topic: 'This is the start of your conversation with Jules.',
        unreadCount: 1,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'jules', 'echo'],
      },
      {
        id: 'dm-mika',
        networkId: 'libera',
        kind: 'direct',
        name: 'mika',
        topic: 'This is the start of your conversation with Mika.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'mika', 'echo'],
      },
      {
        id: 'dm-echo',
        networkId: 'libera',
        kind: 'direct',
        name: 'echo',
        topic: 'A little space to try things out. Echo repeats whatever you send.',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you', 'echo'],
      },
    ],
    cursor: 0,
  }
  let counter = 0
  const today = new Date()
  today.setHours(10, 12, 0, 0)
  const message = (
    conversationId: string,
    senderId: string,
    text: string,
    extra: Partial<Message> = {},
  ): Message => ({
    id: `seed-${++counter}`,
    upstreamId: `demo-${counter}`,
    conversationId,
    senderId,
    text,
    createdAt: new Date(today.getTime() + counter * 60000).toISOString(),
    kind: 'message',
    reactions: [],
    // createDemoSession logs every demo network into its nick's account.
    ...(senderId === 'you' ? { account: 'karbowiak' } : {}),
    ...extra,
  })
  const general = [
    message('general', 'ada', 'ada joined', {
      kind: 'event',
      event: { type: 'join', actorId: 'ada' },
    }),
    message('general', 'lin', 'lin left (Leaving)', {
      kind: 'event',
      event: { type: 'part', actorId: 'lin', text: 'Leaving' },
    }),
    message('general', 'ellis', 'ellis quit (Ping timeout: 245 seconds)', {
      kind: 'event',
      event: { type: 'quit', actorId: 'ellis', text: 'Ping timeout: 245 seconds' },
    }),
    message(
      'general',
      'mika',
      'Morning, everyone! Coffee is on. What’s everyone up to this weekend?',
    ),
    message(
      'general',
      'jules',
      'Finally giving my desk the attention it deserves. The cable situation has reached a point of no return.',
    ),
    message(
      'general',
      'sage',
      'There is no greater weekend project than pretending you’ll finish cable management.',
      { reactions: [{ emoji: '😂', memberIds: ['mika', 'noah', 'you'] }] },
    ),
    message(
      'general',
      'noah',
      'Found this from a road trip earlier this year. A pretty good reminder to get away from the desk.',
      {
        attachment: {
          name: 'a-little-fresh-air.jpg',
          url: '/media/weekend.jpg',
          width: 960,
          height: 1440,
          alt: 'A quiet road winding between red sandstone mountains in Nevada',
        },
      },
    ),
    message('general', 'jules', 'Okay, that’s a pretty convincing argument for going outside.', {
      reactions: [{ emoji: '🌿', memberIds: ['noah', 'sage'] }],
    }),
    message('general', 'you', 'That light is incredible. Where was this?'),
    message('general', 'noah', 'Valley of Fire, in Nevada! We stopped more often than we drove.'),
    message('general', 'sage', 'is now very much planning a road trip', { kind: 'action' }),
    message(
      'general',
      'mika',
      '\x02Reminder:\x02 the \x0306meetup\x03 is on \x1dSaturday\x1d — details in the \x11#general\x11 topic.',
    ),
    message(
      'general',
      'mika',
      'This is your friendly reminder that the best side project is sometimes no side project at all.',
      {
        reactions: [
          { emoji: '💜', memberIds: ['jules', 'sage', 'you'] },
          { emoji: '☕', memberIds: ['noah'] },
        ],
      },
    ),
    message(
      'general',
      'jules',
      'Fine. A walk first, cables later. You’ve all been a terrible influence.',
    ),
    message('general', 'echo', 'Make yourself at home. Send a message and I’ll echo it back. 👋'),
  ]
  const messages: Record<string, Message[]> = { general }
  for (const conversation of bootstrap.conversations) {
    if (messages[conversation.id]) continue
    if (conversation.id === 'dm-jules')
      messages[conversation.id] = [
        message(
          conversation.id,
          'jules',
          'Hey! Good to see you here. Have you tried the new workspace yet?',
        ),
      ]
    else if (conversation.id === 'dm-echo')
      messages[conversation.id] = [
        message(
          conversation.id,
          'echo',
          'Hey there! Send me anything. I’m a good listener, if a little repetitive.',
        ),
      ]
    else if (conversation.id === 'development')
      messages[conversation.id] = [
        message(conversation.id, 'mika', 'What’s everyone working on lately?'),
        message(conversation.id, 'noah', 'A tiny photo journal. Trying very hard to keep it tiny.'),
        message(conversation.id, 'mika', '@karbowiak would love to hear what you’re building too.'),
      ]
    else if (conversation.id === 'music')
      messages[conversation.id] = [
        message(
          conversation.id,
          'lin',
          'A little something for a slow morning: https://musicforprogramming.net',
        ),
        message(conversation.id, 'sage', 'This has been keeping me company all week. Great find.'),
      ]
    else if (conversation.id === 'introductions')
      messages[conversation.id] = [
        message(
          conversation.id,
          'sage',
          'Hello from the other side of the internet! Happy to be here.',
        ),
        message(conversation.id, 'mika', 'Welcome, Sage! Make yourself at home.'),
      ]
    else if (conversation.id === 'design')
      messages[conversation.id] = [
        message(
          conversation.id,
          'jules',
          'Small detail of the day: a good empty state should feel like an invitation.',
        ),
      ]
    else messages[conversation.id] = []
  }
  bootstrap.conversations = bootstrap.conversations.map(wireConversation)
  return { bootstrap, messages }
}
