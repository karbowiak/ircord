# Server presets and WEBIRC

Configure public network presets and optional trusted WEBIRC gateways with a startup-loaded YAML file.
Set `IRCORD_SERVER_LIST_FILE` to the file path and restart the backend after changes.
Without this setting, the client can use its built-in presets.
An explicit empty `servers` list provides no operator presets.

## Contract

The file has separate public `servers` and private `webirc` lists.
Each list accepts at most 20 entries, and the file is limited to 64 KiB.
Unknown fields, duplicate endpoint authorizations, invalid values, and missing credentials stop startup.
Configuration errors do not print rejected values, paths, or secret references.

```yaml
servers:
  - name: Example network
    host: irc.example.net
    port: 6697
    tls: true
    authMethod: sasl-plain
    autoJoin: ["#welcome"]
    color: mint
    initial: E
webirc:
  - host: irc.example.net
    port: 6697
    tls: true
    gateway: ircord
    passwordEnv: IRCORD_WEBIRC_PASSWORD
```

For a mounted secret, use `passwordFile` instead of `passwordEnv`.
Each gateway needs exactly one reference, and its resolved password must be a nonempty IRC parameter.
Do not put passwords in the YAML file.
The backend requires the server list and referenced secret to exist before startup.

Preset `port` defaults to 6697, `tls` to true, and `authMethod` to `none`.
The other authentication methods are `server-password`, `sasl-plain`, and `nickserv`.
`autoJoin`, `color`, and `initial` are optional.
Colors are `lavender`, `mint`, `peach`, `blue`, or `yellow`.
The existing `IRCORD_NETWORK_PASSWORDS` settings remain separate from WEBIRC credentials.
The API reports a supplied server password only for its matching TLS endpoint and authentication method.

Gateway authorization matches the canonical host, port, and TLS setting, independent of the preset name.
The backend sends WEBIRC before registration only when that endpoint is authorized and a verified address exists.
The server must independently accept the gateway credential and connection source.
IRCord's raw WEBIRC command remains blocked.

Gateway credentials use TLS by default.
Plaintext WEBIRC is permitted only with `IRCORD_ALLOW_PRIVATE_IRC=true` and when every resolved destination is private or loopback.
The backend validates the resolved addresses before connecting and connects to the checked address.
It refuses a public or mixed public/private result for plaintext WEBIRC.

## Address selection

The backend uses the authenticated HTTP client's address from the existing trusted-proxy resolver.
Configure `IRCORD_TRUSTED_PROXIES` only for proxies that append the actual peer to `X-Forwarded-For`.
An untrusted peer cannot choose its address with that header.
An invalid or entirely trusted proxy chain does not supply an origin.

Each account keeps its most recently observed verified address privately.
An established IRC connection keeps its original address; another device does not reconnect it.
The next attempt uses the latest observed address, including when no devices remain online.
The first observation is saved before home-network provisioning when possible.
Later observations are kept in memory and coalesced to storage every 30 seconds, on cache eviction, or during a clean shutdown.
A crash can lose up to the most recent 30 seconds of address changes.
Deleting an account removes its stored address; starting without any WEBIRC gateway removes retained addresses.

An account with no verified address connects without WEBIRC until a later attempt has one.
IRCord does not substitute a proxy address or unverified reverse DNS name.
It sends the numeric address as both hostname and address, including valid IPv6 formatting.
The installed IRC library cannot add the optional `secure` flag, so IRCord omits it.
Rejected WEBIRC credentials are not bypassed with an ordinary retry.

The library's raw debug mode can expose incoming server data.
The backend refuses to start with `GIRC_DEBUG` enabled when WEBIRC is configured.
The public presets API never includes gateway credentials, references, or client addresses.
Connection diagnostics, server logs, and command errors redact the gateway password and selected address.
The account's own numeric origin is hidden in member host display data.
IRC messages, URLs, account names, and protocol identifiers keep the values the server sends.
Those values can contain the same text as a password or address.

## Source

- [WEBIRC extension](https://ircv3.net/specs/extensions/webirc)
- [Backend reference](backend.md)
