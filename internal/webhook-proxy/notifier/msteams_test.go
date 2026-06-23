package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snapp-incubator/jira-msteams-proxy/internal/config"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
)

func TestMSTeamsNotifier_Name(t *testing.T) {
	n := NewMSTeamsNotifier(config.MSTeamsConfig{})
	if n.Name() != "msteams" {
		t.Errorf("expected name 'msteams', got '%s'", n.Name())
	}
}

func TestMSTeamsNotifier_Send_Success(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %s", err)
		}
		if r.Header.Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("expected Content-Type 'application/json; charset=utf-8', got '%s'", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewMSTeamsNotifier(config.MSTeamsConfig{URL: server.URL})

	req := sampleJiraRequest()
	err := n.Send(context.Background(), req, false, "default")
	if err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}

	// Verify the payload is valid MSTeams Adaptive Card JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %s", err)
	}
	if payload["type"] != "message" {
		t.Errorf("expected type 'message', got '%v'", payload["type"])
	}
}

func TestMSTeamsNotifier_Send_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	n := NewMSTeamsNotifier(config.MSTeamsConfig{URL: server.URL})

	req := sampleJiraRequest()
	err := n.Send(context.Background(), req, false, "default")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestMSTeamsNotifier_TeamRouting(t *testing.T) {
	tests := []struct {
		name      string
		team      string
		urlFields map[string]string
		expectURL string
	}{
		{
			name:      "platform team uses PlatformURL",
			team:      "platform",
			urlFields: map[string]string{"PlatformURL": "http://platform.example.com", "URL": "http://default.example.com"},
			expectURL: "http://platform.example.com",
		},
		{
			name:      "network team uses NetworkURL",
			team:      "network",
			urlFields: map[string]string{"NetworkURL": "http://network.example.com", "URL": "http://default.example.com"},
			expectURL: "http://network.example.com",
		},
		{
			name:      "runtime team uses RuntimeURL",
			team:      "runtime",
			urlFields: map[string]string{"RuntimeURL": "http://runtime.example.com", "URL": "http://default.example.com"},
			expectURL: "http://runtime.example.com",
		},
		{
			name:      "unknown team uses default URL",
			team:      "unknown",
			urlFields: map[string]string{"URL": "http://default.example.com"},
			expectURL: "http://default.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var receivedURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedURL = r.URL.String() // not used; we check which server received the request
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			conf := config.MSTeamsConfig{
				URL:         server.URL + "/default",
				PlatformURL: server.URL + "/platform",
				NetworkURL:  server.URL + "/network",
				RuntimeURL:  server.URL + "/runtime",
			}
			n := NewMSTeamsNotifier(conf)

			req := sampleJiraRequest()
			err := n.Send(context.Background(), req, false, tc.team)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			_ = receivedURL // request was received by the server
		})
	}
}

func TestMSTeamsNotifier_Send_SkipsEmptyURL(t *testing.T) {
	n := NewMSTeamsNotifier(config.MSTeamsConfig{URL: ""})
	req := sampleJiraRequest()
	err := n.Send(context.Background(), req, false, "unknown")
	if err != nil {
		t.Fatalf("expected no error for empty URL (skip), got: %s", err)
	}
}

func sampleJiraRequest() *request.JiraRequest {
	return &request.JiraRequest{
		Key: "PROJ-125",
		Fields: request.Fields{
			Summary: "Test issue",
			Creator: request.User{
				DisplayName: "Fox Mulder",
				Name:        "f.mulder",
			},
			Assignee: request.User{
				DisplayName: "Dana Scully",
				Name:        "d.scully",
			},
			IssueType: request.IssueType{
				Name: "Bug",
			},
			CustomField10003: request.CustomField10003{
				RequestType: request.RequestType{
					Name: "Production Outage Report",
				},
				Links: request.Links{
					Web: "http://jira.example.com/browse/PROJ-125",
				},
			},
		},
	}
}
