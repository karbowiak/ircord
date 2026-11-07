package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/ergo"
)

func TestOptionalErgoAdministration(t *testing.T) {
	base, _, store, sessions := authenticatedApp(t)
	token := strings.Repeat("ab", 32)
	vendorToken := strings.Repeat("cd", 32)
	verifier, _ := admin.NewVerifier(token)
	var reject atomic.Bool
	var requests atomic.Int32
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+vendorToken || r.Method != "POST" {
			t.Error("invalid vendor authentication")
		}
		if reject.Load() {
			w.WriteHeader(401)
			io.WriteString(w, vendorToken)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "version": "ergo-test", "go_version": "go-test", "start_time": time.Now().Add(-time.Hour).UTC(), "users": map[string]int{"total": 3, "invisible": 1, "operators": 0, "unknown": 0, "max": 5}, "channels": 2, "servers": 1})
	}))
	defer vendor.Close()
	associated, err := ergo.New(ergo.Options{Server: "irc.example.com:6697", URL: vendor.URL, Token: vendorToken})
	if err != nil {
		t.Fatal(err)
	}
	defer associated.Close()
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "associated"}[configured], func(t *testing.T) {
			var integration *ergo.Client
			if configured {
				integration = associated
			}
			app := New(base.Manager, Options{Store: store, Auth: sessions, Admin: verifier, Ergo: integration})
			server := httptest.NewServer(app.Handler)
			defer server.Close()
			get := func(path string, auth bool, want int) []byte {
				t.Helper()
				r, _ := http.NewRequest("GET", server.URL+admin.Prefix+path, nil)
				if auth {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				response, err := http.DefaultClient.Do(r)
				if err != nil {
					t.Fatal(err)
				}
				b, _ := io.ReadAll(response.Body)
				response.Body.Close()
				if response.StatusCode != want {
					t.Fatalf("%s: status %d, want %d", path, response.StatusCode, want)
				}
				if strings.Contains(string(b), vendorToken) || strings.Contains(string(b), vendor.URL) {
					t.Fatal("vendor credential or private location leaked")
				}
				return b
			}
			for _, path := range []string{"/capabilities", "/doctor", "/ergo/status", "/ergo/channels", "/ergo/users/test", "/ergo/accounts", "/ergo/accounts/test"} {
				get(path, false, 401)
			}
			var capabilities admin.AdminCapabilities
			json.Unmarshal(get("/capabilities", true, 200), &capabilities)
			if (len(capabilities.Features) == 1) != configured {
				t.Fatal("incorrect association capabilities")
			}
			before := requests.Load()
			var status admin.Status
			b := get("/status", true, 200)
			json.Unmarshal(b, &status)
			if status.Status != "ok" || status.Overview == nil || status.Overview.DatabaseBytes <= 0 {
				t.Fatal("backend overview missing")
			}
			if configured {
				if status.Ergo == nil || status.Ergo.State != "ok" || status.Ergo.Statistics.Users.Total != 3 {
					t.Fatal("associated status missing")
				}
				reject.Store(true)
				status = admin.Status{}
				json.Unmarshal(get("/status", true, 200), &status)
				if status.Status != "degraded" || status.Database != "available" || status.Overview == nil || status.Ergo.Statistics != nil {
					t.Fatal("failed integration damaged partial backend status")
				}
				reject.Store(false)
			} else {
				if status.Ergo != nil || strings.Contains(string(b), `"ergo"`) {
					t.Fatal("standalone status advertises Ergo")
				}
				get("/ergo/status", true, 404)
				if requests.Load() != before {
					t.Fatal("standalone backend contacted Ergo")
				}
			}
			var doctor admin.Doctor
			json.Unmarshal(get("/doctor", true, 200), &doctor)
			if doctor.Status != "ok" || len(doctor.Checks) != 2+len(capabilities.Features) {
				t.Fatal("incorrect optional diagnostic checks")
			}
			get("/ergo/rehash", true, 404)
		})
	}
}
