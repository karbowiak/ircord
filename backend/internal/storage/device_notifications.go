package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// DeviceNotifications separates desired settings from observed capabilities.
// An uninitialized record is deliberately off until the device imports its
// previous local setting; a remote edit fences out that later import.
type DeviceNotifications struct {
	Revision             uint64     `json:"revision"`
	Explicit             bool       `json:"explicit" doc:"A deliberate settings edit. False for an untouched or migrated local default; remote Off is explicit."`
	Content              string     `json:"content" enum:"inherit,all,mentions,none"`
	Delivery             string     `json:"delivery" enum:"off,open,push"`
	Sound                string     `json:"sound" enum:"off,mentions,all"`
	Preview              bool       `json:"preview"`
	Initialized          bool       `json:"initialized"`
	Permission           string     `json:"permission" enum:"unknown,default,granted,denied,unsupported"`
	Subscription         bool       `json:"subscription" doc:"Backend-observed push registration, not proof of provider delivery."`
	CapabilityReportedAt *time.Time `json:"capabilityReportedAt,omitempty"`
}

type DevicePreferences struct {
	Revision uint64 `json:"revision" doc:"Expected current notification revision. A concurrent edit returns 409."`
	Name     string `json:"name" maxLength:"80"`
	Content  string `json:"content" enum:"inherit,all,mentions,none"`
	Delivery string `json:"delivery" enum:"off,open,push"`
	Sound    string `json:"sound" enum:"off,mentions,all"`
	Preview  bool   `json:"preview"`
	Migrate  bool   `json:"migrate,omitempty" doc:"Import this requesting device's old local choice once. An existing initialized record wins."`
}

type NotificationDevice struct {
	Session, Name, FormFactor string
	LastActiveAt              *time.Time
	Notifications             DeviceNotifications
}

func DefaultDeviceNotifications() DeviceNotifications {
	return DeviceNotifications{Content: "inherit", Delivery: "off", Sound: "mentions", Preview: true, Permission: "unknown"}
}

func (s *Store) NotificationDevice(owner, session string) (NotificationDevice, error) {
	d := NotificationDevice{Session: session, Notifications: DefaultDeviceNotifications()}
	var raw []byte
	var active, capability int64
	var revision uint64
	err := s.DB.QueryRow(`SELECT name,preferences,permission,capability_ns,last_active_ns,form_factor,revision FROM session_notification_settings WHERE owner=? AND session=?`, owner, session).Scan(&d.Name, &raw, &d.Notifications.Permission, &capability, &active, &d.FormFactor, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	permission := d.Notifications.Permission
	if err = json.Unmarshal(raw, &d.Notifications); err != nil {
		return d, err
	}
	d.Notifications.Permission = permission
	d.Notifications.Revision = revision
	if active > 0 {
		at := time.Unix(0, active).UTC()
		d.LastActiveAt = &at
	}
	if capability > 0 {
		at := time.Unix(0, capability).UTC()
		d.Notifications.CapabilityReportedAt = &at
	}
	return d, nil
}

var ErrDevicePreferenceConflict = errors.New("device notification settings changed")

func (s *Store) SetDevicePreferences(owner, session string, p DevicePreferences) error {
	n := DefaultDeviceNotifications()
	n.Content, n.Delivery, n.Sound, n.Preview, n.Initialized = p.Content, p.Delivery, p.Sound, p.Preview, true
	n.Explicit = !p.Migrate
	raw, err := json.Marshal(n)
	if err != nil {
		return err
	}
	query := `INSERT INTO session_notification_settings(session,owner,name,preferences,revision) SELECT id,owner,?,?,1 FROM sessions WHERE owner=? AND id=? AND revoked=0 AND expires_ns>? AND (?=0 OR EXISTS(SELECT 1 FROM session_notification_settings n WHERE n.owner=sessions.owner AND n.session=sessions.id)) ON CONFLICT(session) DO UPDATE SET name=excluded.name,preferences=excluded.preferences,revision=revision+1`
	args := []any{p.Name, raw, owner, session, time.Now().UnixNano(), p.Revision}
	if p.Migrate {
		query += ` WHERE coalesce(json_extract(session_notification_settings.preferences,'$.initialized'),0)=0`
	} else {
		query += ` WHERE revision=?`
		args = append(args, p.Revision)
	}
	result, err := s.DB.Exec(query, args...)
	if err != nil {
		return err
	}
	nrows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if nrows == 0 && !p.Migrate {
		return ErrDevicePreferenceConflict
	}
	return nil
}

func (s *Store) ReportDeviceCapabilities(owner, session, permission, formFactor string, engaged bool, at time.Time) error {
	n := DefaultDeviceNotifications()
	raw, _ := json.Marshal(n)
	active := int64(0)
	if engaged {
		active = at.UnixNano()
	}
	_, err := s.DB.Exec(`INSERT INTO session_notification_settings(session,owner,preferences,permission,capability_ns,last_active_ns,form_factor) SELECT id,owner,?,?,?,?,? FROM sessions WHERE owner=? AND id=? AND revoked=0 AND expires_ns>? ON CONFLICT(session) DO UPDATE SET permission=excluded.permission,capability_ns=excluded.capability_ns,last_active_ns=max(last_active_ns,excluded.last_active_ns),form_factor=CASE WHEN excluded.form_factor='' THEN form_factor ELSE excluded.form_factor END WHERE capability_ns<=excluded.capability_ns`, raw, permission, at.UnixNano(), active, formFactor, owner, session, time.Now().UnixNano())
	return err
}
