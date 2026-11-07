package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPublishedContractAndAuthentication(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	published := request(t, client, "GET", server.URL+"/openapi.json", nil, 200)
	exported, err := os.ReadFile("../../openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected map[string]any
	if json.Unmarshal(published, &actual) != nil || json.Unmarshal(exported, &expected) != nil {
		t.Fatal("invalid contract JSON")
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("served/exported schemas differ; run make api-generate")
	}
	paths := actual["paths"].(map[string]any)
	for path, raw := range paths {
		for method, value := range raw.(map[string]any) {
			if method == "parameters" || method == "summary" || method == "description" {
				continue
			}
			op, ok := value.(map[string]any)
			if !ok {
				continue
			}
			_, secured := op["security"]
			public := path == "/api/v1/auth/session" || path == "/api/v1/auth/login" || path == "/api/v1/auth/register"
			if secured == public {
				t.Fatalf("incorrect security declaration: %s %s", method, path)
			}
		}
	}
	for _, path := range []string{"/api/v1/events", "/api/v1/networks/{id}/connection/trace/events"} {
		if paths[path] == nil {
			t.Fatalf("missing native WebSocket: %s", path)
		}
	}
	response, err := client.Get(server.URL + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(string(body), "@scalar/api-reference@") || !strings.Contains(string(body), "/openapi.json") || !strings.Contains(string(body), "integrity=") {
		t.Fatal("Scalar documentation is not pinned and connected to the schema")
	}
	response, err = client.Get(server.URL + "/api/v1/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var problem struct {
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
	_ = json.NewDecoder(response.Body).Decode(&problem)
	if response.StatusCode != 401 || !strings.Contains(response.Header.Get("Content-Type"), "application/problem+json") || problem.Status != 401 || problem.Detail == "" {
		t.Fatal("unauthenticated resource does not return a typed 401 problem")
	}
	request(t, client, "GET", server.URL+"/api/v1/auth/session", nil, 200)
	register(t, client, server.URL)
	var snapshot map[string]any
	_ = json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200), &snapshot)
	for _, key := range []string{"networks", "conversations", "members"} {
		if _, ok := snapshot[key].([]any); !ok {
			t.Fatalf("empty %s must be an array", key)
		}
	}
	// Browser preflights must permit the notification/settings PUT operations.
	req, _ := http.NewRequest("OPTIONS", server.URL+"/api/v1/notifications/settings", nil)
	req.Header.Set("Origin", server.URL)
	req.Header.Set("Access-Control-Request-Method", "PUT")
	response, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if !strings.Contains(response.Header.Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatal("PUT missing from CORS contract")
	}
}
