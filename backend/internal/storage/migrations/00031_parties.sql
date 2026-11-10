-- +goose Up
CREATE TABLE IF NOT EXISTS parties (
 id TEXT PRIMARY KEY,
 channel TEXT NOT NULL UNIQUE COLLATE NOCASE,
 conversation TEXT NOT NULL UNIQUE,
 creator TEXT NOT NULL,
 client_id TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 9007199254740991),
 payload BLOB NOT NULL,
 UNIQUE(creator,client_id)
);
CREATE TABLE IF NOT EXISTS party_invitations (
 id TEXT PRIMARY KEY,
 party TEXT NOT NULL REFERENCES parties(id),
 invitee TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','accepted','declined','revoked')),
 payload BLOB NOT NULL,
 UNIQUE(party,invitee)
);
CREATE INDEX IF NOT EXISTS party_invitations_account ON party_invitations(invitee,state);
CREATE TABLE IF NOT EXISTS party_history (
 party TEXT NOT NULL REFERENCES parties(id),
 id TEXT NOT NULL,
 created_ns INTEGER NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1,
 payload BLOB NOT NULL,
 PRIMARY KEY(party,id)
);
CREATE INDEX IF NOT EXISTS party_history_page ON party_history(party,created_ns,id);
CREATE INDEX IF NOT EXISTS party_history_replies ON party_history(party,json_extract(payload,'$.replyTo.id'));
CREATE TABLE IF NOT EXISTS party_copies (party TEXT NOT NULL,owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,message TEXT NOT NULL,revision INTEGER NOT NULL,PRIMARY KEY(party,owner,message),FOREIGN KEY(party,message) REFERENCES party_history(party,id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS party_control (id INTEGER PRIMARY KEY CHECK(id=1),payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS party_identities (account_id TEXT PRIMARY KEY,username TEXT NOT NULL UNIQUE COLLATE NOCASE,payload BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS party_participants (party TEXT NOT NULL REFERENCES parties(id),account_id TEXT NOT NULL,username TEXT NOT NULL,PRIMARY KEY(party,account_id));
CREATE TABLE IF NOT EXISTS party_history_sync (party TEXT PRIMARY KEY REFERENCES parties(id),generation TEXT NOT NULL,before_ns INTEGER NOT NULL,complete INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS party_uploads (upload TEXT NOT NULL,party TEXT NOT NULL REFERENCES parties(id),PRIMARY KEY(upload,party));
CREATE INDEX IF NOT EXISTS party_file_scope ON message_links(file_id,conversation) WHERE file_id!='';
CREATE TABLE IF NOT EXISTS party_reactions (party TEXT NOT NULL REFERENCES parties(id),parent TEXT NOT NULL,sender TEXT NOT NULL,emoji TEXT NOT NULL,active INTEGER NOT NULL,at_ns INTEGER NOT NULL,PRIMARY KEY(party,parent,sender,emoji));
