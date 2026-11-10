package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ircord/backend/internal/model"
)

// PartyRecord includes transport and recovery state which is never a public DTO.
type PartyRecord struct {
	model.Party
	Channel         string   `json:"channel"`
	Creator         string   `json:"creator"`
	ClientID        string   `json:"clientId"`
	Fingerprint     string   `json:"fingerprint"`
	Revoked         []string `json:"revoked,omitempty"`
	IdentityVersion int      `json:"identityVersion,omitempty"`
}

func (p PartyRecord) HasMember(owner string) bool {
	for _, v := range p.Members {
		if v.PrincipalID() == owner {
			return true
		}
	}
	return false
}

func partyRow(row *sql.Row) (PartyRecord, error) {
	var p PartyRecord
	var raw []byte
	e := row.Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(raw, &p)
	}
	return p, e
}
func (s *Store) Party(id string) (PartyRecord, error) {
	return partyRow(s.DB.QueryRow("SELECT payload FROM parties WHERE id=?", id))
}
func (s *Store) PartyByConversation(id string) (PartyRecord, error) {
	return partyRow(s.DB.QueryRow("SELECT payload FROM parties WHERE conversation=?", id))
}
func (s *Store) PartyByTarget(target string) (PartyRecord, error) {
	return partyRow(s.DB.QueryRow("SELECT payload FROM parties WHERE channel=? COLLATE NOCASE", target))
}
func (s *Store) PartyByRequest(owner, client string) (PartyRecord, error) {
	return partyRow(s.DB.QueryRow("SELECT payload FROM parties WHERE creator=? AND client_id=?", owner, client))
}
func (s *Store) Parties() (out []PartyRecord, err error) {
	out = []PartyRecord{}
	rows, err := s.DB.Query("SELECT payload FROM parties ORDER BY id")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var p PartyRecord
		if err = rows.Scan(&raw); err != nil {
			return
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			return
		}
		out = append(out, p)
	}
	err = rows.Err()
	return
}

var ErrPartyInvitationLimit = errors.New("party invitation limit reached")

func (s *Store) CreateParty(p PartyRecord, invitations ...PartyInviteRecord) error {
	for _, member := range p.Members {
		if e := s.SavePartyIdentity(member.PartyCandidate); e != nil {
			return e
		}
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, i := range invitations {
		var pending int
		var eligible bool
		if e = tx.QueryRow("SELECT count(*) FROM party_invitations i JOIN parties p ON p.id=i.party WHERE i.invitee=? AND i.state='pending' AND json_extract(p.payload,'$.state')!='archived'", i.Invitee).Scan(&pending); e != nil {
			return e
		}
		if e = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM party_services_identities WHERE id=?) OR EXISTS(SELECT 1 FROM accounts WHERE id=? AND deleted=0 AND username IS NOT NULL AND last_seen_ns>?)", i.Invitee, i.Invitee, time.Now().Add(-AccountRetention).UnixNano()).Scan(&eligible); e != nil {
			return e
		}
		if pending >= 20 || !eligible {
			return ErrPartyInvitationLimit
		}
	}
	if _, e = tx.Exec("INSERT INTO parties(id,channel,conversation,creator,client_id,fingerprint,revision,payload) VALUES(?,?,?,?,?,?,?,?)", p.ID, p.Channel, p.ConversationID, p.Creator, p.ClientID, p.Fingerprint, p.Revision, raw); e != nil {
		return e
	}
	for _, i := range invitations {
		raw, e := json.Marshal(i.Invitation)
		if e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO party_invitations(id,party,invitee,state,payload) VALUES(?,?,?,'pending',?)", i.Invitation.ID, p.ID, i.Invitee, raw); e != nil {
			return e
		}
	}
	for _, member := range p.Members {
		if _, e = tx.Exec("INSERT INTO party_participants(party,account_id,username) VALUES(?,?,?) ON CONFLICT DO NOTHING", p.ID, member.PrincipalID(), member.Username); e != nil {
			return e
		}
	}
	return tx.Commit()
}

// SaveParty compares revisions so interrupted or concurrent workflows cannot
// overwrite a newer membership decision. The caller serializes server effects.
func (s *Store) SaveParty(p PartyRecord, previous int64) error {
	for _, member := range p.Members {
		if e := s.SavePartyIdentity(member.PartyCandidate); e != nil {
			return e
		}
	}
	if p.Revision != previous+1 || p.Revision > 9007199254740991 {
		return errors.New("invalid party revision")
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	r, e := s.DB.Exec("UPDATE parties SET revision=?,payload=? WHERE id=? AND revision=?", p.Revision, raw, p.ID, previous)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n != 1 {
		return errors.New("party changed during update")
	}
	if e == nil {
		for _, member := range p.Members {
			if _, e = s.DB.Exec("INSERT INTO party_participants(party,account_id,username) VALUES(?,?,?) ON CONFLICT DO NOTHING", p.ID, member.PrincipalID(), member.Username); e != nil {
				return e
			}
		}
	}
	return e
}

func (s *Store) PartyCandidate(id string) (model.PartyCandidate, error) {
	if strings.HasPrefix(id, "irc:") {
		return s.PartyServiceIdentity(id)
	}
	var v model.PartyCandidate
	var raw []byte
	e := s.DB.QueryRow("SELECT a.id,a.username,COALESCE(e.payload,'{}') FROM accounts a LEFT JOIN entities e ON e.owner=a.id AND e.kind='member' AND e.id=a.id WHERE a.id=? AND a.deleted=0 AND a.username IS NOT NULL AND a.last_seen_ns>?", id, time.Now().Add(-AccountRetention).UnixNano()).Scan(&v.AccountID, &v.Username, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	partyCandidateProfile(&v, raw)
	return v, nil
}

func partyCandidateProfile(v *model.PartyCandidate, raw []byte) {
	var member model.Member
	_ = json.Unmarshal(raw, &member)
	v.DisplayName, v.AvatarURL = member.DisplayName, member.AvatarURL
	if v.DisplayName == "" || v.DisplayName == "ircord" {
		v.DisplayName = v.Username
	}
}

func (s *Store) PartyCandidates(search string) ([]model.PartyCandidate, error) {
	rows, e := s.DB.Query(`WITH search(value) AS (VALUES(lower(?))), candidates AS (
		SELECT a.id,a.username,COALESCE(e.payload,'{}') AS profile,
			COALESCE(CASE WHEN json_valid(e.payload) THEN
				NULLIF(NULLIF(json_extract(e.payload,'$.displayName'),''),'ircord')
			END,a.username) AS display_name
		FROM accounts a LEFT JOIN entities e ON e.owner=a.id AND e.kind='member' AND e.id=a.id
		WHERE a.deleted=0 AND a.username IS NOT NULL AND a.last_seen_ns>?
	)
	SELECT id,username,profile FROM candidates CROSS JOIN search
	WHERE instr(lower(username),value)>0 OR instr(lower(display_name),value)>0
	ORDER BY CASE
		WHEN lower(username)=value THEN 0
		WHEN lower(display_name)=value THEN 1
		WHEN instr(lower(username),value)=1 THEN 2
		WHEN instr(lower(display_name),value)=1 THEN 3
		ELSE 4 END,username COLLATE NOCASE,id
	LIMIT 50`, strings.TrimSpace(search), time.Now().Add(-AccountRetention).UnixNano())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.PartyCandidate{}
	for rows.Next() {
		var v model.PartyCandidate
		var raw []byte
		if e = rows.Scan(&v.AccountID, &v.Username, &raw); e != nil {
			return nil, e
		}
		partyCandidateProfile(&v, raw)
		out = append(out, v)
	}
	return out, rows.Err()
}

type PartyInviteRecord struct {
	Invitation     model.Invitation
	Invitee, State string
}

func (s *Store) PartyInvitation(owner, id string) (PartyInviteRecord, error) {
	owner = s.PartyPrincipal(owner)
	var v PartyInviteRecord
	var raw []byte
	e := s.DB.QueryRow("SELECT invitee,state,payload FROM party_invitations WHERE invitee=? AND id=?", owner, id).Scan(&v.Invitee, &v.State, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(raw, &v.Invitation)
	}
	return v, e
}
func (s *Store) PutPartyInvitation(party, owner string, v model.Invitation) (PartyInviteRecord, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return PartyInviteRecord{}, e
	}
	// A declined invite remains a durable mask. No member can reset it by
	// sending the same invitation under a new request ID.
	_, e = s.DB.Exec("INSERT INTO party_invitations(id,party,invitee,state,payload) SELECT ?,?,?,'pending',? WHERE (SELECT count(*) FROM party_invitations i JOIN parties p ON p.id=i.party WHERE i.invitee=? AND i.state='pending' AND json_extract(p.payload,'$.state')!='archived')<20 AND ((SELECT count(*) FROM party_invitations WHERE party=?)<256 OR EXISTS(SELECT 1 FROM party_invitations WHERE party=? AND invitee=?)) ON CONFLICT(party,invitee) DO UPDATE SET id=excluded.id,state='pending',payload=excluded.payload WHERE party_invitations.state='revoked'", v.ID, party, owner, raw, owner, party, party, owner)
	if e != nil {
		return PartyInviteRecord{}, e
	}
	var id string
	e = s.DB.QueryRow("SELECT id FROM party_invitations WHERE party=? AND invitee=?", party, owner).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return PartyInviteRecord{}, errors.New("invitation limit reached")
	}
	if e != nil {
		return PartyInviteRecord{}, e
	}
	return s.PartyInvitation(owner, id)
}
func (s *Store) SetPartyInvitation(owner, id, state string) error {
	owner = s.PartyPrincipal(owner)
	_, e := s.DB.Exec("UPDATE party_invitations SET state=? WHERE invitee=? AND id=? AND state='pending'", state, owner, id)
	return e
}

func (s *Store) RevokePartyInvitation(party, owner string) error {
	owner = s.PartyPrincipal(owner)
	_, e := s.DB.Exec("UPDATE party_invitations SET state='revoked' WHERE party=? AND invitee=? AND state IN ('pending','accepted')", party, owner)
	return e
}
func (s *Store) PendingPartyInvitations(owner string) ([]model.Invitation, error) {
	owner = s.PartyPrincipal(owner)
	rows, e := s.DB.Query("SELECT i.payload FROM party_invitations i JOIN parties p ON p.id=i.party WHERE i.invitee=? AND i.state='pending' AND json_extract(p.payload,'$.state')!='archived' ORDER BY i.id", owner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Invitation{}
	for rows.Next() {
		var raw []byte
		var v model.Invitation
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) PartyInvitations(party string) ([]PartyInviteRecord, error) {
	rows, e := s.DB.Query("SELECT invitee,state,payload FROM party_invitations WHERE party=?", party)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PartyInviteRecord{}
	for rows.Next() {
		var v PartyInviteRecord
		var raw []byte
		if e = rows.Scan(&v.Invitee, &v.State, &raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v.Invitation); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) PartyControl() ([]byte, error) {
	var raw []byte
	e := s.DB.QueryRow("SELECT payload FROM party_control WHERE id=1").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	return s.OpenSecret("party-controller", "identity", raw)
}
func (s *Store) SavePartyControl(raw []byte) error {
	_, e := s.DB.Exec("INSERT INTO party_control(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", s.SealSecret("party-controller", "identity", raw))
	return e
}

// Party history is bounded by the same 90-day account retention horizon and a
// hard per-party ceiling. Copies use the identical message IDs and revisions.
const PartyHistoryLimit = 50000

func (s *Store) SavePartyMessage(party string, msg model.Message) error {
	if msg.UpstreamID == "" {
		return nil
	}
	msg.ClientID, msg.Account = "", ""
	msg.Historical = false
	raw, e := json.Marshal(msg)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO party_history(party,id,created_ns,payload) VALUES(?,?,?,?) ON CONFLICT(party,id) DO UPDATE SET payload=excluded.payload,revision=party_history.revision+1 WHERE party_history.payload!=excluded.payload", party, msg.ID, msg.CreatedAt.UnixNano(), raw); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE party_history SET payload=json_set(payload,'$.replyTo.text',?),revision=revision+1 WHERE party=? AND json_extract(payload,'$.replyTo.id')=? AND json_extract(payload,'$.replyTo.text')!=?", msg.Text, party, msg.ID, msg.Text); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) PartyMessage(party, id string) (model.Message, error) {
	var v model.Message
	var raw []byte
	e := s.DB.QueryRow("SELECT payload FROM party_history WHERE party=? AND (id=? OR json_extract(payload,'$.upstreamId')=?)", party, id, id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(raw, &v)
	}
	return v, e
}
func (s *Store) PartyHistory(party string, after int64, limit int) ([]model.Message, error) {
	rows, e := s.DB.Query("SELECT payload FROM party_history WHERE party=? AND created_ns>=? AND created_ns>? ORDER BY created_ns,id LIMIT ?", party, time.Now().Add(-AccountRetention).UnixNano(), after, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var raw []byte
		var v model.Message
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) MaterializeParty(owner string, p PartyRecord) error {
	principal := s.PartyPrincipal(owner)
	// Caller owns the party lock; membership cannot change during this transaction.
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var currentRaw []byte
	if e = tx.QueryRow("SELECT payload FROM parties WHERE id=?", p.ID).Scan(&currentRaw); e != nil {
		return e
	}
	var current PartyRecord
	if e = json.Unmarshal(currentRaw, &current); e != nil {
		return e
	}
	if !current.HasMember(principal) || current.Revision != p.Revision {
		return ErrNotFound
	}
	rows, e := tx.Query("SELECT h.payload,h.revision,COALESCE(json_extract(m.payload,'$.clientId'),'') FROM party_history h LEFT JOIN party_copies c ON c.party=h.party AND c.owner=? AND c.message=h.id LEFT JOIN messages m ON m.owner=? AND m.id=h.id WHERE h.party=? AND h.created_ns>=? AND (c.revision IS NULL OR c.revision<h.revision OR m.id IS NULL) ORDER BY h.created_ns,h.id", owner, owner, p.ID, time.Now().Add(-AccountRetention).UnixNano())
	if e != nil {
		return e
	}
	messages := []model.Message{}
	versions := []int64{}
	for rows.Next() {
		var raw []byte
		var v model.Message
		var version int64
		var clientID string
		if e = rows.Scan(&raw, &version, &clientID); e != nil {
			rows.Close()
			return e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			rows.Close()
			return e
		}
		v.Historical = true
		v.ClientID = clientID
		for _, member := range p.Members {
			if member.AccountID == owner && strings.EqualFold(member.Username, v.SenderAccount) {
				v.Account = v.SenderAccount
			}
		}
		messages = append(messages, v)
		versions = append(versions, version)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for i, v := range messages {
		if e = putMessage(tx, owner, v); e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO party_copies(party,owner,message,revision) VALUES(?,?,?,?) ON CONFLICT(party,owner,message) DO UPDATE SET revision=excluded.revision", p.ID, owner, v.ID, versions[i]); e != nil {
			return e
		}
	}
	if _, e = tx.Exec("DELETE FROM messages WHERE owner=? AND conversation=? AND upstream!='' AND NOT EXISTS(SELECT 1 FROM party_history h WHERE h.party=? AND h.id=messages.id)", owner, p.ConversationID, p.ID); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) PurgePartyCopies(owner, conversation string) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("DELETE FROM party_copies WHERE owner=? AND party IN (SELECT id FROM parties WHERE conversation=?)", owner, conversation); e != nil {
		return e
	}
	var notifications bool
	if e = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='notification_candidates')").Scan(&notifications); e != nil {
		return e
	}
	if notifications {
		if _, e = tx.Exec("DELETE FROM notification_candidates WHERE owner=? AND conversation=?", owner, conversation); e != nil {
			return e
		}
	}
	for _, table := range []string{"messages", "outbox", "history_state", "conversation_read", "reactions", "message_mentions", "message_links", "saved_messages", "inbox_items"} {
		if _, e = tx.Exec("DELETE FROM "+table+" WHERE owner=? AND conversation=?", owner, conversation); e != nil {
			return e
		}
	}
	if _, e = tx.Exec("DELETE FROM entities WHERE owner=? AND kind='conversation' AND id=?", owner, conversation); e != nil {
		return e
	}
	// Old frames may carry private bytes. Force a snapshot after revocation.
	if _, e = tx.Exec("DELETE FROM events WHERE owner=?", owner); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) BindPartyUpload(upload, party string) error {
	_, e := s.DB.Exec("INSERT INTO party_uploads(upload,party) VALUES(?,?) ON CONFLICT DO NOTHING", upload, party)
	return e
}
