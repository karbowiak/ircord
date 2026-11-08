export interface paths {
    "/api/v1/admin/capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Discover configured administration features */
        get: operations["admin-capabilities"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/doctor": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Check backend and optional associated-server health */
        get: operations["admin-doctor"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/ergo/accounts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List associated-server NickServ accounts */
        get: operations["admin-ergo-accounts"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/ergo/accounts/{name}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Inspect an associated-server NickServ account */
        get: operations["admin-ergo-account"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/ergo/channels": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List associated-server channels */
        get: operations["admin-ergo-channels"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/ergo/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Inspect the associated Ergo server */
        get: operations["admin-ergo-status"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/ergo/users/{name}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Inspect an associated-server nickname */
        get: operations["admin-ergo-user"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Inspect backend status
         * @description Operator-only aggregate backend health, with optional associated-Ergo statistics. An unassociated backend omits Ergo entirely; failed checks retain the available sections and mark status degraded. No credentials or private API location is returned.
         */
        get: operations["admin-status"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/app-data/{kind}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 app data by kind */
        get: operations["get-api-v1-app-data-by-kind"];
        put: operations["save-app-data"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/account": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete: operations["delete-account"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/login": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["login"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["logout"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/password": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["change-password"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/registration": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 auth registration */
        get: operations["get-api-v1-auth-registration"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/session": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 auth session */
        get: operations["get-api-v1-auth-session"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/signup": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["signup"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/username": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 auth username */
        get: operations["get-api-v1-auth-username"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/bootstrap": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 bootstrap */
        get: operations["get-api-v1-bootstrap"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/commands/{clientId}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 commands by client ID */
        get: operations["get-api-v1-commands-by-client-id"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 conversations */
        post: operations["post-api-v1-conversations"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 conversations by ID */
        delete: operations["delete-api-v1-conversations-by-id"];
        options?: never;
        head?: never;
        /** Patch API v1 conversations by ID */
        patch: operations["patch-api-v1-conversations-by-id"];
        trace?: never;
    };
    "/api/v1/conversations/{id}/access": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["get-channel-access"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/access/{account}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put: operations["set-channel-access"];
        post?: never;
        delete: operations["remove-channel-access"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/channel-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Change channel-wide settings
         * @description Changes the notification level and server link previews for everyone in the channel. Needs channel operator status or higher; every joined IRCord member receives the new values.
         */
        put: operations["update-channel-settings"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/lists/{mode}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 conversations by ID lists by mode */
        get: operations["get-api-v1-conversations-by-id-lists-by-mode"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/messages": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 conversations by ID messages */
        get: operations["get-api-v1-conversations-by-id-messages"];
        put?: never;
        post: operations["send-message"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/messages/{messageId}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /**
         * Delete one of your messages for everyone
         * @description Sends an IRC REDACT through draft/message-redaction. Only the author may delete a message. The canonical change is delivered as message.updated with deleted set; uploaded files are kept.
         */
        delete: operations["redact-message"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 conversations by ID preview */
        get: operations["get-api-v1-conversations-by-id-preview"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/reactions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 conversations by ID reactions */
        post: operations["post-api-v1-conversations-by-id-reactions"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/registration": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["get-channel-registration"];
        put?: never;
        post: operations["register-channel"];
        delete: operations["unregister-channel"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/conversations/{id}/transfer": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["transfer-channel"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/emotes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Read the instance emote catalog
         * @description Every account on this IRCord backend shares the catalog. Images are served by IRCord; rendering never contacts a provider.
         */
        get: operations["get-emotes"];
        put?: never;
        /**
         * Upload a custom emote
         * @description Requires emote management. The image is bounded, MIME-sniffed and stored as an instance asset that outlives the uploader's account.
         */
        post: operations["upload-emote"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/emotes/imports": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Import a provider emote
         * @description Requires emote management. The provider's image is validated and copied into IRCord's storage; its ID, link and creator are kept for attribution.
         */
        post: operations["import-emote"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/emotes/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /**
         * Remove an emote
         * @description Requires emote management. Messages that used it keep their readable :shortcode: text.
         */
        delete: operations["delete-emote"];
        options?: never;
        head?: never;
        /**
         * Rename, re-alias, enable or disable an emote
         * @description Requires emote management. Names an emote no longer has are retired: old messages still render it, and another emote may claim them.
         */
        patch: operations["update-emote"];
        trace?: never;
    };
    "/api/v1/events": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Open the authenticated event WebSocket
         * @description Upgrade to WebSocket. Send session.resume using the last applied durable cursor. Typing and emotes.updated events have no sequence and are never replayed; session.ready carries the current emote catalog version instead. Send typing.set for activity; use HTTP for durable commands.
         */
        get: operations["stream-events"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/events/replay": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 events replay */
        get: operations["get-api-v1-events-replay"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/hash-field/scores": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List Hash Field high scores
         * @description The instance-wide top 10 of the Hash Field game in Settings → About.
         */
        get: operations["get-hash-field-scores"];
        put?: never;
        /**
         * Submit a Hash Field score
         * @description Enters a finished game's score into the instance-wide top 10. A score that does not place is not stored. One account may submit once every 10 seconds and 20 times an hour.
         */
        post: operations["submit-hash-field-score"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/invitations/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 invitations by ID */
        delete: operations["delete-api-v1-invitations-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 networks */
        post: operations["post-api-v1-networks"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/check": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Post API v1 networks check */
        post: operations["post-api-v1-networks-check"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 networks by ID */
        delete: operations["delete-api-v1-networks-by-id"];
        options?: never;
        head?: never;
        /** Patch API v1 networks by ID */
        patch: operations["patch-api-v1-networks-by-id"];
        trace?: never;
    };
    "/api/v1/networks/{id}/channels": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 networks by ID channels */
        get: operations["get-api-v1-networks-by-id-channels"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}/commands": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["send-raw"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}/connection": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 networks by ID connection */
        get: operations["get-api-v1-networks-by-id-connection"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}/connection/trace": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 networks by ID connection trace */
        get: operations["get-api-v1-networks-by-id-connection-trace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}/connection/trace/events": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Open the receive-only diagnostic WebSocket */
        get: operations["stream-connection-trace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}/self": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        /** Patch API v1 networks by ID self */
        patch: operations["patch-api-v1-networks-by-id-self"];
        trace?: never;
    };
    "/api/v1/networks/{id}/server-log": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 networks by ID server log */
        get: operations["get-api-v1-networks-by-id-server-log"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/networks/{id}/uploads": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 networks by ID uploads */
        get: operations["get-api-v1-networks-by-id-uploads"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/notifications/settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 notifications settings */
        get: operations["get-api-v1-notifications-settings"];
        /** Put API v1 notifications settings */
        put: operations["put-api-v1-notifications-settings"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/profile": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        /** Patch API v1 profile */
        patch: operations["patch-api-v1-profile"];
        trace?: never;
    };
    "/api/v1/push/key": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 push key */
        get: operations["get-api-v1-push-key"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/push/subscriptions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["save-push-subscription"];
        /** Delete API v1 push subscriptions */
        delete: operations["delete-api-v1-push-subscriptions"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/push/test": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["test-push"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/saved-message-ids": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List which messages you saved
         * @description References only, bounded by the per-account limit, so a client can show the saved state on any message.
         */
        get: operations["list-saved-message-ids"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/saved-messages": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List your saved messages
         * @description Newest first. Each item is resolved against your current history: deleted messages keep only their author and time, and messages of conversations you left are unavailable.
         */
        get: operations["list-saved-messages"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/saved-messages/{messageId}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Save a message for later
         * @description Idempotent: saving a message you already saved returns the existing bookmark. The message must be a stored message of the conversation, named by its canonical ID. Nobody else is notified and the message is unchanged.
         */
        put: operations["save-message"];
        post?: never;
        /**
         * Remove a saved message
         * @description Removes only your bookmark, by canonical message ID. Removing one that is not saved succeeds.
         */
        delete: operations["unsave-message"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/search": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get API v1 search */
        get: operations["get-api-v1-search"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/uploads": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Upload an account-owned file
         * @description Requires the session cookie. Optional networkId selects and scopes the filehost. Files are bounded and MIME-sniffed by the streaming handler.
         */
        post: operations["upload-file"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/uploads/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete API v1 uploads by ID */
        delete: operations["delete-api-v1-uploads-by-id"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        AccessChangeBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/AccessChangeBody.json
             */
            readonly $schema?: string;
            /** @enum {string} */
            mode: "a" | "o" | "h" | "v";
        };
        AccessEntry: {
            account: string;
            /** @enum {string} */
            mode: "q" | "a" | "o" | "h" | "v";
        };
        Account: {
            /** Format: date-time */
            createdAt: string;
            /** Format: date-time */
            expiresAt: string;
            id: string;
            /** Format: date-time */
            lastSeenAt: string;
            username: string;
        };
        AdminCapabilities: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/AdminCapabilities.json
             */
            readonly $schema?: string;
            features: string[];
        };
        AdminOverview: {
            /** Format: int64 */
            accounts: number;
            /**
             * Format: int64
             * @description SQLite allocated page bytes, excluding WAL and uploaded files.
             */
            databaseBytes: number;
            /**
             * Format: int64
             * @description Currently connected upstream IRC sessions, not IRC users or accounts.
             */
            ircConnections: number;
            /** Format: int64 */
            messages: number;
            /** Format: int64 */
            networks: number;
            /** Format: int64 */
            sessions: number;
        };
        AppDocument: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/AppDocument.json
             */
            readonly $schema?: string;
            data: unknown;
            revision: string;
        };
        AppearanceUpdatedEvent: {
            data: components["schemas"]["RevisionUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "appearance.updated";
            /** @enum {integer} */
            v: 1;
        };
        Attachment: {
            alt: string;
            contentType?: string;
            /** Format: int64 */
            height: number;
            name: string;
            /** Format: int64 */
            size?: number;
            url: string;
            /** Format: int64 */
            width: number;
        };
        AuthInput: {
            account?: string;
            /** @enum {string} */
            method: "none" | "sasl-plain" | "sasl-external" | "server-password" | "nickserv";
            password?: string;
        };
        BrowserSubscription: {
            endpoint: string;
            /** Format: double */
            expirationTime?: number | null;
            keys: components["schemas"]["Keys"];
        };
        Capabilities: {
            serverPreviews: boolean;
            upload?: components["schemas"]["UploadCapability"];
        };
        ChannelListEntry: {
            mask: string;
            /** Format: date-time */
            setAt?: string;
            setBy?: string;
        };
        ChannelListing: {
            name: string;
            topic: string;
            /** Format: int64 */
            users: number;
        };
        ChannelRegistration: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ChannelRegistration.json
             */
            readonly $schema?: string;
            founder?: string;
            registered: boolean;
            /** Format: date-time */
            registeredAt?: string;
        };
        ChannelServicesChange: {
            conversationId: string;
        };
        ChannelServicesUpdatedEvent: {
            data: components["schemas"]["ChannelServicesChange"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "channel-services.updated";
            /** @enum {integer} */
            v: 1;
        };
        ChannelSettings: {
            /**
             * @description Channel notification level for every member; default uses each member's own channel default.
             * @enum {string}
             */
            notifications: "default" | "all" | "mentions" | "none";
            /**
             * @description Whether IRCord's server fetches link previews for this channel.
             * @enum {string}
             */
            previewMode: "client" | "server";
            /** Format: date-time */
            updatedAt?: string;
            /** @description IRC nick of the operator who last changed these settings. */
            updatedBy?: string;
        };
        ChannelSettingsPatchBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ChannelSettingsPatchBody.json
             */
            readonly $schema?: string;
            /** @enum {string} */
            notifications?: "default" | "all" | "mentions" | "none";
            /** @enum {string} */
            previewMode?: "client" | "server";
        };
        ClientEvent: components["schemas"]["SessionResume"] | components["schemas"]["TypingCommand"];
        CommandError: {
            clientId?: string;
            message: string;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "command.error";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        CommandResponse: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/CommandResponse.json
             */
            readonly $schema?: string;
            clientId: string;
            conversationId: string;
            error?: string;
            message?: components["schemas"]["Message"];
            messageId?: string;
            /** @enum {string} */
            state: "pending" | "sent" | "unconfirmed" | "uncertain" | "failed";
        };
        ConnectionInfo: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ConnectionInfo.json
             */
            readonly $schema?: string;
            /** Format: date-time */
            connectedAt: string | null;
            isupport: {
                [key: string]: string;
            };
            lastError?: string;
            motd: string[];
            negotiatedCapabilities: string[];
            networkId: string;
            offeredCapabilities: {
                [key: string]: string;
            };
            serverName: string;
            state: string;
            tls?: components["schemas"]["TLSInfo"];
            upstreamFilehostUrl?: string;
        };
        ConnectionUpdatedEvent: {
            data: components["schemas"]["ConnectionInfo"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "connection.updated";
            /** @enum {integer} */
            v: 1;
        };
        Conversation: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Conversation.json
             */
            readonly $schema?: string;
            channelSettings?: components["schemas"]["ChannelSettings"];
            /** Format: date-time */
            createdAt?: string;
            favorite: boolean;
            id: string;
            joined: boolean;
            /** @enum {string} */
            kind: "channel" | "direct";
            memberIds: string[];
            memberModes: {
                [key: string]: string;
            };
            /** Format: int64 */
            mentionCount: number;
            modeParams?: {
                [key: string]: string;
            };
            modes: string;
            name: string;
            networkId: string;
            /**
             * @description Effective preview mode. For channels it follows channelSettings; for direct messages it is personal.
             * @enum {string}
             */
            previewMode?: "client" | "server";
            target?: string;
            topic: string;
            /** Format: date-time */
            topicSetAt?: string;
            topicSetBy?: string;
            /** Format: int64 */
            unreadCount: number;
            /** @description Message a manual mark-as-unread starts at; cleared when the conversation is read again */
            unreadFrom?: string;
            url?: string;
        };
        ConversationInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ConversationInputBody.json
             */
            readonly $schema?: string;
            key?: string;
            memberId?: string;
            name?: string;
            networkId: string;
        };
        ConversationPatchBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ConversationPatchBody.json
             */
            readonly $schema?: string;
            favorite?: boolean;
            /**
             * @description Personal preview mode for a direct message. Channels use channel-settings.
             * @enum {string}
             */
            previewMode?: "client" | "server";
            read?: boolean;
            /** @description Mark the conversation unread from this message on */
            unreadFrom?: string;
        };
        ConversationRemovedEvent: {
            data: components["schemas"]["Removal"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "conversation.removed";
            /** @enum {integer} */
            v: 1;
        };
        ConversationUpdatedEvent: {
            data: components["schemas"]["Conversation"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "conversation.updated";
            /** @enum {integer} */
            v: 1;
        };
        Credentials: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Credentials.json
             */
            readonly $schema?: string;
            password: string;
            username: string;
        };
        "Delete-api-v1-push-subscriptionsRequest": {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Delete-api-v1-push-subscriptionsRequest.json
             */
            readonly $schema?: string;
            endpoint: string;
        };
        Doctor: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Doctor.json
             */
            readonly $schema?: string;
            checks: components["schemas"]["DoctorCheck"][];
            /** @enum {string} */
            status: "ok" | "degraded";
        };
        DoctorCheck: {
            detail: string;
            name: string;
            /** @enum {string} */
            status: "ok" | "unavailable";
        };
        Emote: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Emote.json
             */
            readonly $schema?: string;
            /** @description Further names that render and complete to this emote. */
            aliases: string[] | null;
            animated: boolean;
            /** Format: date-time */
            createdAt: string;
            /** @description Attribution: the provider's creator, or one given for an upload. */
            creator?: string;
            enabled: boolean;
            /** Format: int64 */
            height: number;
            /** @description Stable instance emote ID. Clients namespace it as instance:<id>. */
            id: string;
            /** @description Canonical name, such as the provider's original casing. */
            name: string;
            providerId?: string;
            /** @description Former names. They still render in message history but are not offered for new messages, and a new emote may claim them. */
            retired: string[] | null;
            /** @description Lowercase :shortcode: written into message text. */
            shortcode: string;
            /** @enum {string} */
            source: "upload" | "7tv" | "ffz";
            sourceUrl?: string;
            /** @description Still frame, used when animation is off. Equals url for still images. */
            staticUrl: string;
            /** Format: date-time */
            updatedAt: string;
            url: string;
            /** Format: int64 */
            width: number;
        };
        EmoteCatalog: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/EmoteCatalog.json
             */
            readonly $schema?: string;
            /** @description Whether this account may upload, import, edit and remove emotes. */
            canManage: boolean;
            /** @description Enabled entries; managers also receive disabled ones. */
            items: components["schemas"]["Emote"][] | null;
            /** Format: int64 */
            maxBytes: number;
            providers: string[] | null;
            types: string[] | null;
            /**
             * Format: int64
             * @description Changes whenever the catalog changes; emotes.updated carries the new value.
             */
            version: number;
        };
        EmoteImport: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/EmoteImport.json
             */
            readonly $schema?: string;
            aliases?: string[] | null;
            /** @enum {string} */
            provider: "7tv" | "ffz";
            providerId: string;
            /** @description Defaults to the provider's name in lowercase. */
            shortcode?: string;
        };
        EmotePatch: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/EmotePatch.json
             */
            readonly $schema?: string;
            aliases?: string[];
            creator?: string;
            enabled?: boolean;
            name?: string;
            shortcode?: string;
        };
        EmoteVersion: {
            /** Format: int64 */
            version: number;
        };
        EmotesUpdatedEvent: {
            data: components["schemas"]["EmoteVersion"];
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "emotes.updated";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        ErgoAccount: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ErgoAccount.json
             */
            readonly $schema?: string;
            channels: string[];
            name: string;
            /** Format: date-time */
            registeredAt?: string;
        };
        ErgoChannel: {
            hasKey: boolean;
            inviteOnly: boolean;
            name: string;
            owner?: string;
            registered: boolean;
            secret: boolean;
            topic: string;
            /** Format: int64 */
            users: number;
        };
        ErgoStatistics: {
            /** Format: int64 */
            channels: number;
            goVersion: string;
            /** Format: int64 */
            servers: number;
            /** Format: date-time */
            startedAt: string;
            /** Format: int64 */
            uptimeSeconds: number;
            users: components["schemas"]["ErgoUsers"];
            version: string;
        };
        ErgoStatus: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ErgoStatus.json
             */
            readonly $schema?: string;
            /** Format: date-time */
            checkedAt: string;
            error?: string;
            server: string;
            /** @enum {string} */
            state: "ok" | "unavailable";
            statistics?: components["schemas"]["ErgoStatistics"];
        };
        ErgoUser: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ErgoUser.json
             */
            readonly $schema?: string;
            account?: string;
            away?: string;
            channels: components["schemas"]["ErgoUserChannel"][];
            modes?: string;
            nickname: string;
            realname?: string;
            /** Format: int64 */
            sessions: number;
        };
        ErgoUserChannel: {
            mode?: string;
            name: string;
        };
        ErgoUsers: {
            /** Format: int64 */
            invisible: number;
            /** Format: int64 */
            operators: number;
            /** Format: int64 */
            peak: number;
            /** Format: int64 */
            total: number;
            /** Format: int64 */
            unknown: number;
        };
        ErrorDetail: {
            /** @description Where the error occurred, e.g. 'body.items[3].tags' or 'path.thing-id' */
            location?: string;
            /** @description Error message text */
            message?: string;
            /** @description The value at the given location */
            value?: unknown;
        };
        ErrorModel: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ErrorModel.json
             */
            readonly $schema?: string;
            /**
             * @description A human-readable explanation specific to this occurrence of the problem.
             * @example Property foo is required but is missing.
             */
            detail?: string;
            /** @description Optional list of individual error details */
            errors?: components["schemas"]["ErrorDetail"][] | null;
            /**
             * Format: uri
             * @description A URI reference that identifies the specific occurrence of the problem.
             * @example https://example.com/error-log/abc123
             */
            instance?: string;
            /**
             * Format: int64
             * @description HTTP status code
             * @example 400
             */
            status?: number;
            /**
             * @description A short, human-readable summary of the problem type. This value should not change between occurrences of the error.
             * @example Bad Request
             */
            title?: string;
            /**
             * Format: uri
             * @description A URI reference to human-readable documentation for the error.
             * @default about:blank
             * @example https://example.com/errors/example
             */
            type: string;
        };
        Event: components["schemas"]["MessageCreatedEvent"] | components["schemas"]["InvitationReceivedEvent"] | components["schemas"]["InvitationRemovedEvent"] | components["schemas"]["MessageUpdatedEvent"] | components["schemas"]["ConversationUpdatedEvent"] | components["schemas"]["ChannelServicesUpdatedEvent"] | components["schemas"]["NetworkUpdatedEvent"] | components["schemas"]["ProfileUpdatedEvent"] | components["schemas"]["MemberUpdatedEvent"] | components["schemas"]["ConversationRemovedEvent"] | components["schemas"]["NetworkRemovedEvent"] | components["schemas"]["HistorySyncedEvent"] | components["schemas"]["HistoryGapEvent"] | components["schemas"]["HistoryFailedEvent"] | components["schemas"]["ConnectionUpdatedEvent"] | components["schemas"]["ServerLineEvent"] | components["schemas"]["ServerLogUpdatedEvent"] | components["schemas"]["AppearanceUpdatedEvent"] | components["schemas"]["LibraryUpdatedEvent"] | components["schemas"]["PreferencesUpdatedEvent"] | components["schemas"]["NotificationsUpdatedEvent"] | components["schemas"]["SavedAddedEvent"] | components["schemas"]["SavedRemovedEvent"];
        Features: {
            away: boolean;
            history: boolean;
            /** Format: int64 */
            messageLineBytes: number;
            metadata: string[];
            multiline?: components["schemas"]["MultilineLimits"];
            reactions: boolean;
            readMarkers: boolean;
            redact: boolean;
            replies: boolean;
            setname: boolean;
            typing: boolean;
        };
        GIF: {
            /** Format: int64 */
            height: number;
            id: string;
            previewUrl: string;
            /** @enum {string} */
            provider: "klipy";
            slug: string;
            stillUrl: string;
            title: string;
            url: string;
            /** Format: int64 */
            width: number;
        };
        HashFieldEntry: {
            name: string;
            /**
             * Format: int64
             * @description Place in the table, starting at 1
             */
            rank: number;
            /** Format: int64 */
            score: number;
        };
        HashFieldResult: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/HashFieldResult.json
             */
            readonly $schema?: string;
            /** @description The table after the submission, best first */
            items: components["schemas"]["HashFieldEntry"][] | null;
            /** @description Whether the score made the table */
            placed: boolean;
            /**
             * Format: int64
             * @description The new entry's place, or 0 when it did not place
             */
            rank: number;
        };
        HashFieldScoreInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/HashFieldScoreInputBody.json
             */
            readonly $schema?: string;
            /** @description Name shown in the table; trimmed, at most 24 characters */
            name: string;
            /**
             * Format: int64
             * @description Final score of one game
             */
            score: number;
        };
        HashFieldTable: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/HashFieldTable.json
             */
            readonly $schema?: string;
            /** @description The top entries, best first */
            items: components["schemas"]["HashFieldEntry"][] | null;
        };
        HistoryFailedEvent: {
            data: components["schemas"]["HistoryUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "history.sync.failed";
            /** @enum {integer} */
            v: 1;
        };
        HistoryGapEvent: {
            data: components["schemas"]["HistoryUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "history.gap";
            /** @enum {integer} */
            v: 1;
        };
        HistorySyncedEvent: {
            data: components["schemas"]["HistoryUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "history.synced";
            /** @enum {integer} */
            v: 1;
        };
        HistoryUpdate: {
            conversationId: string;
            message?: string;
            reason?: string;
        };
        Invitation: {
            channel: string;
            /** Format: date-time */
            createdAt: string;
            id: string;
            inviterId: string;
            inviterNick: string;
            networkId: string;
        };
        InvitationReceivedEvent: {
            data: components["schemas"]["Invitation"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "invitation.received";
            /** @enum {integer} */
            v: 1;
        };
        InvitationRemovedEvent: {
            data: components["schemas"]["Removal"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "invitation.removed";
            /** @enum {integer} */
            v: 1;
        };
        ItemsAccessEntry: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsAccessEntry.json
             */
            readonly $schema?: string;
            items: components["schemas"]["AccessEntry"][];
        };
        ItemsChannelListEntry: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsChannelListEntry.json
             */
            readonly $schema?: string;
            items: components["schemas"]["ChannelListEntry"][];
        };
        ItemsChannelListing: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsChannelListing.json
             */
            readonly $schema?: string;
            items: components["schemas"]["ChannelListing"][];
        };
        ItemsErgoAccount: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsErgoAccount.json
             */
            readonly $schema?: string;
            items: components["schemas"]["ErgoAccount"][];
        };
        ItemsErgoChannel: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsErgoChannel.json
             */
            readonly $schema?: string;
            items: components["schemas"]["ErgoChannel"][];
        };
        ItemsSavedMessage: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsSavedMessage.json
             */
            readonly $schema?: string;
            items: components["schemas"]["SavedMessage"][];
        };
        ItemsSearchResult: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ItemsSearchResult.json
             */
            readonly $schema?: string;
            items: components["schemas"]["SearchResult"][];
        };
        Keys: {
            auth: string;
            p256dh: string;
        };
        LibraryUpdatedEvent: {
            data: components["schemas"]["RevisionUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "library.updated";
            /** @enum {integer} */
            v: 1;
        };
        LinkPreview: {
            author?: string;
            description?: string;
            embed?: components["schemas"]["PreviewEmbed"];
            image?: string;
            /** Format: int64 */
            imageHeight?: number;
            /** Format: int64 */
            imageWidth?: number;
            images?: components["schemas"]["PreviewImage"][] | null;
            /** @enum {string} */
            kind?: "website" | "article" | "video";
            media?: components["schemas"]["PreviewMedia"];
            siteName?: string;
            title: string;
            url: string;
        };
        LoginResult: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/LoginResult.json
             */
            readonly $schema?: string;
            account: components["schemas"]["Account"];
        };
        Member: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Member.json
             */
            readonly $schema?: string;
            account: string;
            avatarUrl?: string;
            bio: string;
            color: string;
            displayName: string;
            host?: string;
            id: string;
            nameColor?: string;
            /** @enum {string} */
            nameSource?: "nick" | "realname" | "profile";
            nick: string;
            realname?: string;
            /** @enum {string} */
            role: "member" | "operator" | "bot";
            /** @enum {string} */
            status: "online" | "away" | "offline";
            username?: string;
        };
        MemberUpdatedEvent: {
            data: components["schemas"]["Member"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "member.updated";
            /** @enum {integer} */
            v: 1;
        };
        MembershipEvent: {
            actorId: string;
            targetId?: string;
            text?: string;
            /** @enum {string} */
            type: "join" | "part" | "quit" | "nick" | "kick" | "mode" | "topic" | "invite";
        };
        Message: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Message.json
             */
            readonly $schema?: string;
            account?: string;
            attachment?: components["schemas"]["Attachment"];
            clientId?: string;
            conversationId: string;
            /** Format: date-time */
            createdAt: string;
            deleted?: boolean;
            event?: components["schemas"]["MembershipEvent"];
            gif?: components["schemas"]["GIF"];
            historical?: boolean;
            id: string;
            /** @enum {string} */
            kind: "message" | "system" | "action" | "notice" | "event";
            /** Format: int64 */
            position?: number;
            preview?: components["schemas"]["LinkPreview"];
            /** @enum {string} */
            previewSource?: "client" | "server";
            reactions: components["schemas"]["Reaction"][];
            replyTo?: components["schemas"]["Reply"];
            senderId: string;
            text: string;
            upstreamId?: string;
        };
        MessageCreatedEvent: {
            data: components["schemas"]["Message"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "message.created";
            /** @enum {integer} */
            v: 1;
        };
        MessageInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/MessageInputBody.json
             */
            readonly $schema?: string;
            attachments?: string[];
            clientId: string;
            gif?: components["schemas"]["GIF"];
            /** @enum {string} */
            kind?: "message" | "action";
            replyToId?: string;
            text: string;
        };
        MessagePage: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/MessagePage.json
             */
            readonly $schema?: string;
            hasNewer?: boolean;
            items: components["schemas"]["Message"][];
            nextCursor: string | null;
            syncError?: string;
        };
        MessageUpdatedEvent: {
            data: components["schemas"]["Message"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "message.updated";
            /** @enum {integer} */
            v: 1;
        };
        MultilineLimits: {
            /** Format: int64 */
            maxBytes: number;
            /** Format: int64 */
            maxLines: number;
        };
        Network: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Network.json
             */
            readonly $schema?: string;
            /** @description Account reported by the connected IRC server, independent of saved login settings. */
            account?: string;
            altNick?: string;
            auth: components["schemas"]["NetworkAuth"];
            autoJoin: string[];
            capabilities: string[];
            color: string;
            features: components["schemas"]["Features"];
            host: string;
            id: string;
            initials: string;
            isupport: {
                [key: string]: string;
            };
            name: string;
            nick: string;
            perform: string[];
            /** Format: int64 */
            port: number;
            realname?: string;
            /** @enum {string} */
            status: "connected" | "disconnected" | "connecting" | "error";
            statusDetail?: string;
            tls: boolean;
            username?: string;
        };
        NetworkAuth: {
            account?: string;
            /** @enum {string} */
            method: "none" | "sasl-plain" | "sasl-external" | "server-password" | "nickserv";
            passwordSet: boolean;
        };
        NetworkCheck: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/NetworkCheck.json
             */
            readonly $schema?: string;
            detail: string;
            reachable: boolean;
        };
        NetworkCheckInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/NetworkCheckInputBody.json
             */
            readonly $schema?: string;
            host: string;
            /** Format: int64 */
            port: number;
            tls: boolean;
        };
        NetworkConfig: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/NetworkConfig.json
             */
            readonly $schema?: string;
            altNick?: string;
            auth?: components["schemas"]["AuthInput"];
            autoJoin?: string[] | null;
            channels?: string[] | null;
            host: string;
            id?: string;
            name: string;
            nick: string;
            perform?: string[] | null;
            /** Format: int64 */
            port: number;
            realname?: string;
            saslPassword?: string;
            saslUser?: string;
            serverPassword?: string;
            tls: boolean;
            username?: string;
        };
        NetworkInvalidation: {
            networkId: string;
        };
        NetworkRemovedEvent: {
            data: components["schemas"]["Removal"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "network.removed";
            /** @enum {integer} */
            v: 1;
        };
        NetworkUpdate: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/NetworkUpdate.json
             */
            readonly $schema?: string;
            altNick?: string;
            auth?: components["schemas"]["AuthInput"];
            autoJoin?: string[];
            host?: string;
            name?: string;
            nick?: string;
            perform?: string[];
            /** Format: int64 */
            port?: number;
            realname?: string;
            /** @enum {string} */
            status?: "connected" | "disconnected";
            tls?: boolean;
            username?: string;
        };
        NetworkUpdatedEvent: {
            data: components["schemas"]["Network"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "network.updated";
            /** @enum {integer} */
            v: 1;
        };
        NotificationsUpdatedEvent: {
            data: components["schemas"]["RevisionUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "notification-settings.updated";
            /** @enum {integer} */
            v: 1;
        };
        "OutputStruct { ClientID string \"json:\\\"clientId\\\"\" }Body": {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/OutputStruct { ClientID string "json:\"clientId\"" }Body.json
             */
            readonly $schema?: string;
            clientId: string;
        };
        "OutputStruct { Items ListEvent \"json:\\\"items\\\"\"; Valid bool \"json:\\\"valid\\\"\" }Body": {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/OutputStruct { Items ListEvent "json:\"items\""; Valid bool "json:\"valid\"" }Body.json
             */
            readonly $schema?: string;
            items: components["schemas"]["Event"][] | null;
            valid: boolean;
        };
        PasswordChange: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/PasswordChange.json
             */
            readonly $schema?: string;
            current: string;
            password: string;
        };
        PreferencesUpdatedEvent: {
            data: components["schemas"]["RevisionUpdate"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "client-preferences.updated";
            /** @enum {integer} */
            v: 1;
        };
        PreviewEmbed: {
            embeddable: boolean;
            id: string;
            /** @enum {string} */
            provider: "youtube";
            short?: boolean;
            /** Format: int64 */
            start?: number;
            unavailable?: boolean;
        };
        PreviewImage: {
            /** Format: int64 */
            height?: number;
            url: string;
            /** Format: int64 */
            width?: number;
        };
        PreviewMedia: {
            /** Format: int64 */
            height?: number;
            /** @enum {string} */
            kind: "gif";
            url: string;
            /** Format: int64 */
            width?: number;
        };
        PreviewResult: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/PreviewResult.json
             */
            readonly $schema?: string;
            /** @enum {string} */
            mode: "client" | "server";
            preview: components["schemas"]["LinkPreview"] | null;
            reason?: string;
        };
        ProfileUpdate: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ProfileUpdate.json
             */
            readonly $schema?: string;
            avatarDataUrl?: string | null;
            avatarUrl?: string | null;
            bio: string;
            /** @description Explicit profile name. Omit to keep the current one; send an empty string or null to clear it and fall back to the network real name or nickname. */
            displayName?: string | null;
            nameColor?: string | null;
        };
        ProfileUpdatedEvent: {
            data: components["schemas"]["Member"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "profile.updated";
            /** @enum {integer} */
            v: 1;
        };
        Provider: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Provider.json
             */
            readonly $schema?: string;
            available: boolean;
            canDelete: boolean;
            /** Format: int64 */
            maxBytes: number;
            provider: string;
            reason?: string;
            types: string[] | null;
        };
        PushKey: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/PushKey.json
             */
            readonly $schema?: string;
            enabled: boolean;
            publicKey: string;
        };
        QuietHours: {
            enabled: boolean;
            end: string;
            start: string;
            timeZone: string;
        };
        Reaction: {
            emoji: string;
            memberIds: string[];
        };
        ReactionInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ReactionInputBody.json
             */
            readonly $schema?: string;
            emoji: string;
            messageId: string;
        };
        RedactResponse: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/RedactResponse.json
             */
            readonly $schema?: string;
            /** @description True when the deleted message carried an attachment. Deleting a message never deletes the uploaded file. */
            attachmentRetained: boolean;
            message: components["schemas"]["Message"];
            /** @description The caller's own upload behind that attachment, which DELETE /api/v1/uploads/{id} removes. */
            uploadId?: string;
        };
        RegistrationPolicy: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/RegistrationPolicy.json
             */
            readonly $schema?: string;
            registrationEnabled: boolean;
            registrationKeyRequired: boolean;
        };
        Removal: {
            id: string;
        };
        Reply: {
            id: string;
            nick: string;
            text: string;
        };
        RevisionUpdate: {
            revision: string;
        };
        "Save-push-subscriptionRequest": {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Save-push-subscriptionRequest.json
             */
            readonly $schema?: string;
            device?: string;
            subscription: components["schemas"]["BrowserSubscription"];
        };
        SaveInputBody: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SaveInputBody.json
             */
            readonly $schema?: string;
            conversationId: string;
        };
        SavedAddedEvent: {
            data: components["schemas"]["SavedMessage"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "saved.added";
            /** @enum {integer} */
            v: 1;
        };
        SavedItem: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SavedItem.json
             */
            readonly $schema?: string;
            conversation?: components["schemas"]["Conversation"];
            conversationId: string;
            message?: components["schemas"]["Message"];
            messageId: string;
            networkId: string;
            /** Format: date-time */
            savedAt: string;
            /** @enum {string} */
            status: "available" | "deleted" | "unavailable";
        };
        SavedMessage: {
            conversationId: string;
            messageId: string;
            networkId: string;
            /** Format: date-time */
            savedAt: string;
        };
        SavedPage: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SavedPage.json
             */
            readonly $schema?: string;
            items: components["schemas"]["SavedItem"][];
            nextCursor: string | null;
        };
        SavedRemoval: {
            conversationId: string;
            messageId: string;
        };
        SavedRemovedEvent: {
            data: components["schemas"]["SavedRemoval"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "saved.removed";
            /** @enum {integer} */
            v: 1;
        };
        SearchResult: {
            conversation: components["schemas"]["Conversation"];
            message: components["schemas"]["Message"];
        };
        SelfUpdate: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SelfUpdate.json
             */
            readonly $schema?: string;
            away?: string | null;
            nick?: string;
            realname?: string;
        };
        "Send-rawRequest": {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Send-rawRequest.json
             */
            readonly $schema?: string;
            clientId: string;
            line: string;
        };
        ServerEvent: components["schemas"]["MessageCreatedEvent"] | components["schemas"]["InvitationReceivedEvent"] | components["schemas"]["InvitationRemovedEvent"] | components["schemas"]["MessageUpdatedEvent"] | components["schemas"]["ConversationUpdatedEvent"] | components["schemas"]["ChannelServicesUpdatedEvent"] | components["schemas"]["NetworkUpdatedEvent"] | components["schemas"]["ProfileUpdatedEvent"] | components["schemas"]["MemberUpdatedEvent"] | components["schemas"]["ConversationRemovedEvent"] | components["schemas"]["NetworkRemovedEvent"] | components["schemas"]["HistorySyncedEvent"] | components["schemas"]["HistoryGapEvent"] | components["schemas"]["HistoryFailedEvent"] | components["schemas"]["ConnectionUpdatedEvent"] | components["schemas"]["ServerLineEvent"] | components["schemas"]["ServerLogUpdatedEvent"] | components["schemas"]["AppearanceUpdatedEvent"] | components["schemas"]["LibraryUpdatedEvent"] | components["schemas"]["PreferencesUpdatedEvent"] | components["schemas"]["NotificationsUpdatedEvent"] | components["schemas"]["SavedAddedEvent"] | components["schemas"]["SavedRemovedEvent"] | components["schemas"]["SessionReady"] | components["schemas"]["SessionResync"] | components["schemas"]["CommandError"] | components["schemas"]["TypingEvent"] | components["schemas"]["EmotesUpdatedEvent"];
        ServerLine: {
            command: string;
            id: string;
            /** @enum {string} */
            kind: "numeric" | "notice" | "error" | "motd" | "raw" | "outgoing";
            networkId: string;
            params: string[];
            source?: string;
            tags?: {
                [key: string]: string;
            };
            text: string;
            /** Format: date-time */
            time: string;
        };
        ServerLineEvent: {
            data: components["schemas"]["ServerLine"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "server.line";
            /** @enum {integer} */
            v: 1;
        };
        ServerLogPage: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/ServerLogPage.json
             */
            readonly $schema?: string;
            items: components["schemas"]["ServerLine"][];
            nextCursor: string | null;
        };
        ServerLogUpdatedEvent: {
            data: components["schemas"]["NetworkInvalidation"];
            seq: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "server-log.updated";
            /** @enum {integer} */
            v: 1;
        };
        SessionReady: {
            /** Format: int64 */
            cursor: number;
            /** Format: int64 */
            emoteVersion?: number;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "session.ready";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        SessionResume: {
            /** Format: int64 */
            after: number;
            /** @enum {string} */
            type: "session.resume";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        SessionResync: {
            reason: string;
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "session.resync";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        Settings: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Settings.json
             */
            readonly $schema?: string;
            /** @enum {string} */
            channelDefault: "all" | "mentions" | "none";
            /** @enum {string} */
            directDefault: "all" | "mentions" | "none";
            highlightWords: string[] | null;
            ignore: string[] | null;
            levels: {
                [key: string]: "all" | "mentions" | "none";
            };
            quietHours: components["schemas"]["QuietHours"];
        };
        SignupCredentials: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/SignupCredentials.json
             */
            readonly $schema?: string;
            /** @description How the account appears in chat; optional, and changeable later. */
            displayName?: string;
            password: string;
            registrationKey?: string;
            username: string;
        };
        Snapshot: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Snapshot.json
             */
            readonly $schema?: string;
            capabilities: components["schemas"]["Capabilities"];
            conversations: components["schemas"]["Conversation"][];
            /** Format: int64 */
            cursor: number;
            demo: boolean;
            invitations: components["schemas"]["Invitation"][];
            members: components["schemas"]["Member"][];
            networks: components["schemas"]["Network"][];
            user: components["schemas"]["Member"];
            /**
             * Format: int64
             * @enum {integer}
             */
            version: 1;
        };
        Status: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Status.json
             */
            readonly $schema?: string;
            commit: string;
            /** @enum {string} */
            database: "available" | "unavailable";
            ergo?: components["schemas"]["ErgoStatus"];
            goVersion: string;
            overview?: components["schemas"]["AdminOverview"];
            /** Format: date-time */
            startedAt: string;
            /** @enum {string} */
            status: "ok" | "degraded";
            /** Format: int64 */
            uptimeSeconds: number;
            version: string;
        };
        TLSInfo: {
            certificateSha256?: string;
            cipher: string;
            serverName: string;
            verified: boolean;
            version: string;
        };
        TraceEvent: components["schemas"]["TraceLine"] | components["schemas"]["TraceResync"];
        TraceLine: {
            data: components["schemas"]["ServerLine"];
            /** @enum {string} */
            type: "connection.line";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        TraceResync: {
            reason: string;
            /** @enum {string} */
            type: "connection.resync";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        "Transfer-channelRequest": {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Transfer-channelRequest.json
             */
            readonly $schema?: string;
            account: string;
        };
        TransferResult: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/TransferResult.json
             */
            readonly $schema?: string;
            /** @enum {string} */
            status: "pending" | "done";
        };
        TypingCommand: {
            conversationId: string;
            /** @enum {string} */
            state: "active" | "paused" | "done";
            /** @enum {string} */
            type: "typing.set";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        TypingData: {
            conversationId: string;
            memberId: string;
            /** @enum {string} */
            state: "active" | "paused" | "done";
        };
        TypingEvent: {
            data: components["schemas"]["TypingData"];
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            type: "typing";
            /**
             * Format: int64
             * @enum {integer}
             */
            v: 1;
        };
        Upload: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/Upload.json
             */
            readonly $schema?: string;
            contentType: string;
            /** Format: int64 */
            height?: number;
            id: string;
            name: string;
            /** Format: int64 */
            size: number;
            url: string;
            /** Format: int64 */
            width?: number;
        };
        UploadCapability: {
            /** Format: int64 */
            maxBytes: number;
            types: string[];
        };
        UsernameCheck: {
            /**
             * Format: uri
             * @description A URL to the JSON Schema for this object.
             * @example https://example.com/schemas/UsernameCheck.json
             */
            readonly $schema?: string;
            available: boolean;
            reason?: string;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    "admin-capabilities": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AdminCapabilities"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-doctor": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Doctor"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-ergo-accounts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsErgoAccount"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-ergo-account": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                name: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErgoAccount"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-ergo-channels": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsErgoChannel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-ergo-status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErgoStatus"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-ergo-user": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                name: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErgoUser"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "admin-status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Status"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-app-data-by-kind": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                kind: "appearance" | "library" | "preferences";
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AppDocument"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "save-app-data": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                kind: "appearance" | "library" | "preferences";
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AppDocument"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AppDocument"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-account": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    login: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Credentials"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LoginResult"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    logout: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "change-password": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["PasswordChange"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-auth-registration": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RegistrationPolicy"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-auth-session": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        account: components["schemas"]["Account"] | null;
                    };
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    signup: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SignupCredentials"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LoginResult"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-auth-username": {
        parameters: {
            query?: {
                name?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["UsernameCheck"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-bootstrap": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Snapshot"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-commands-by-client-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                clientId: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CommandResponse"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "post-api-v1-conversations": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ConversationInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Conversation"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-api-v1-conversations-by-id": {
        parameters: {
            query?: {
                reason?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "patch-api-v1-conversations-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ConversationPatchBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Conversation"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-channel-access": {
        parameters: {
            query?: {
                reason?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsAccessEntry"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "set-channel-access": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                account: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AccessChangeBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsAccessEntry"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "remove-channel-access": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                account: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsAccessEntry"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "update-channel-settings": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ChannelSettingsPatchBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Conversation"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-conversations-by-id-lists-by-mode": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                mode: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsChannelListEntry"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-conversations-by-id-messages": {
        parameters: {
            query?: {
                before?: string;
                /** @description Read a window centred on this message ID instead of the newest page; cannot be combined with before */
                around?: string;
                limit?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["MessagePage"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "send-message": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["MessageInputBody"];
            };
        };
        responses: {
            /** @description Accepted */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CommandResponse"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "redact-message": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
                messageId: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RedactResponse"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-conversations-by-id-preview": {
        parameters: {
            query?: {
                url?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PreviewResult"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "post-api-v1-conversations-by-id-reactions": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ReactionInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Message"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-channel-registration": {
        parameters: {
            query?: {
                reason?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ChannelRegistration"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "register-channel": {
        parameters: {
            query?: {
                reason?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ChannelRegistration"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "unregister-channel": {
        parameters: {
            query?: {
                reason?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "transfer-channel": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Transfer-channelRequest"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TransferResult"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-emotes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["EmoteCatalog"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "upload-emote": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "multipart/form-data": {
                    /** @description Comma-separated aliases. */
                    aliases?: string;
                    /** @description Optional attribution. */
                    creator?: string;
                    /**
                     * Format: binary
                     * @description PNG, GIF, WebP or JPEG image.
                     */
                    file: Blob;
                    /** @description Canonical name; defaults to the shortcode as typed. */
                    name?: string;
                    shortcode: string;
                    /**
                     * Format: binary
                     * @description Still frame for an animated WebP or PNG; a GIF's first frame is used otherwise.
                     */
                    still?: Blob;
                };
            };
        };
        responses: {
            /** @description The new emote */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Emote"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "import-emote": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EmoteImport"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Emote"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-emote": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "update-emote": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["EmotePatch"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Emote"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "stream-events": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description WebSocket upgraded */
            101: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-events-replay": {
        parameters: {
            query?: {
                after?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct { Items ListEvent \"json:\\\"items\\\"\"; Valid bool \"json:\\\"valid\\\"\" }Body"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-hash-field-scores": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HashFieldTable"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "submit-hash-field-score": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["HashFieldScoreInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HashFieldResult"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-api-v1-invitations-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "post-api-v1-networks": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["NetworkConfig"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Network"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "post-api-v1-networks-check": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["NetworkCheckInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["NetworkCheck"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-api-v1-networks-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "patch-api-v1-networks-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["NetworkUpdate"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Network"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-networks-by-id-channels": {
        parameters: {
            query?: {
                q?: string;
                limit?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsChannelListing"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "send-raw": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Send-rawRequest"];
            };
        };
        responses: {
            /** @description Accepted */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["OutputStruct { ClientID string \"json:\\\"clientId\\\"\" }Body"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-networks-by-id-connection": {
        parameters: {
            query?: {
                reason?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionInfo"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-networks-by-id-connection-trace": {
        parameters: {
            query?: {
                before?: string;
                limit?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ServerLogPage"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "stream-connection-trace": {
        parameters: {
            query?: {
                after?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description WebSocket upgraded; connection.line and connection.resync frames. */
            101: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "patch-api-v1-networks-by-id-self": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SelfUpdate"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Network"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-networks-by-id-server-log": {
        parameters: {
            query?: {
                before?: string;
                limit?: number;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ServerLogPage"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-networks-by-id-uploads": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Provider"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-notifications-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Settings"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "put-api-v1-notifications-settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Settings"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Settings"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "patch-api-v1-profile": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ProfileUpdate"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Member"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-push-key": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PushKey"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "save-push-subscription": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Save-push-subscriptionRequest"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-api-v1-push-subscriptions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Delete-api-v1-push-subscriptionsRequest"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "test-push": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "list-saved-message-ids": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsSavedMessage"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "list-saved-messages": {
        parameters: {
            query?: {
                cursor?: string;
                /** @description Keep only saved messages whose text contains this, ignoring ASCII letter case */
                q?: string;
                limit?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SavedPage"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "save-message": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Canonical IRCord message ID (Message.id), not an upstream msgid */
                messageId: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SaveInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SavedItem"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "unsave-message": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Canonical IRCord message ID (Message.id), not an upstream msgid */
                messageId: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "get-api-v1-search": {
        parameters: {
            query?: {
                q?: string;
                conversationId?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ItemsSearchResult"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "upload-file": {
        parameters: {
            query?: {
                networkId?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "multipart/form-data": {
                    /** Format: binary */
                    file: Blob;
                };
            };
        };
        responses: {
            /** @description Uploaded file metadata */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Upload"];
                };
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
    "delete-api-v1-uploads-by-id": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Request failed; see the problem detail. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            422: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            429: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            501: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Request failed; see the problem detail. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
            /** @description Error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/problem+json": components["schemas"]["ErrorModel"];
                };
            };
        };
    };
}
