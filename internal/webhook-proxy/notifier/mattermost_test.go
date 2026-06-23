package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
)

func TestMattermostNotifier_Name(t *testing.T) {
	n := NewMattermostNotifier("http://example.com/hook")
	if n.Name() != "mattermost" {
		t.Errorf("expected name 'mattermost', got '%s'", n.Name())
	}
}

func TestMattermostNotifier_Send_Success(t *testing.T) {
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

	n := NewMattermostNotifier(server.URL)
	req := sampleJiraRequest()

	err := n.Send(context.Background(), req, false, "platform")
	if err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}

	// Parse and validate payload structure
	var payload MattermostPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %s", err)
	}

	if len(payload.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
	}

	att := payload.Attachments[0]
	if att.Color != "#36a64f" {
		t.Errorf("expected green color '#36a64f' for issues, got '%s'", att.Color)
	}
	if att.TitleLink == "" {
		t.Error("expected title_link to be set")
	}
	if len(att.Fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(att.Fields))
	}

	// Validate field titles
	expectedTitles := []string{"Type", "Summary", "Issuer", "Assignee"}
	for i, title := range expectedTitles {
		if att.Fields[i].Title != title {
			t.Errorf("field %d: expected title '%s', got '%s'", i, title, att.Fields[i].Title)
		}
	}
}

func TestMattermostNotifier_Send_CommentColor(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewMattermostNotifier(server.URL)
	req := sampleJiraRequest()

	err := n.Send(context.Background(), req, true, "platform")
	if err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}

	var payload MattermostPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %s", err)
	}

	att := payload.Attachments[0]
	if att.Color != "#2196F3" {
		t.Errorf("expected blue color '#2196F3' for comments, got '%s'", att.Color)
	}
}

func TestMattermostNotifier_Send_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("webhook error"))
	}))
	defer server.Close()

	n := NewMattermostNotifier(server.URL)
	req := sampleJiraRequest()

	err := n.Send(context.Background(), req, false, "default")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestMattermostNotifier_PayloadFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewMattermostNotifier(server.URL)

	tests := []struct {
		name        string
		req         *request.JiraRequest
		isComment   bool
		wantTitle   string
		wantColor   string
		wantTypeVal string
	}{
		{
			name: "issue with full data",
			req: &request.JiraRequest{
				Key: "PROJ-100",
				Fields: request.Fields{
					Summary: "Test summary",
					Creator: request.User{DisplayName: "Alice"},
					Assignee: request.User{DisplayName: "Bob"},
					CustomField10003: request.CustomField10003{
						RequestType: request.RequestType{Name: "Bug Report"},
						Links:       request.Links{Web: "http://jira.example.com/PROJ-100"},
					},
				},
			},
			isComment:   false,
			wantTitle:   "🎯 PROJ-100",
			wantColor:   "#36a64f",
			wantTypeVal: "Bug Report",
		},
		{
			name: "comment",
			req: &request.JiraRequest{
				Key: "PROJ-200",
				Fields: request.Fields{
					Summary: "Comment summary",
					Creator: request.User{DisplayName: "Charlie"},
					Assignee: request.User{},
					CustomField10003: request.CustomField10003{
						RequestType: request.RequestType{Name: "Task"},
						Links:       request.Links{Web: "http://jira.example.com/PROJ-200"},
					},
				},
			},
			isComment:   true,
			wantTitle:   "📰 New Comment on PROJ-200",
			wantColor:   "#2196F3",
			wantTypeVal: "Task",
		},
		{
			name: "empty optional fields",
			req: &request.JiraRequest{
				Key: "PROJ-300",
				Fields: request.Fields{
					Summary: "Minimal",
				},
			},
			isComment:   false,
			wantTitle:   "🎯 PROJ-300",
			wantColor:   "#36a64f",
			wantTypeVal: "N/A",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := n.buildPayload(tc.req, tc.isComment)

			if len(payload.Attachments) != 1 {
				t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
			}
			att := payload.Attachments[0]

			if att.Title != tc.wantTitle {
				t.Errorf("expected title '%s', got '%s'", tc.wantTitle, att.Title)
			}
			if att.Color != tc.wantColor {
				t.Errorf("expected color '%s', got '%s'", tc.wantColor, att.Color)
			}
			if att.Fields[0].Value != tc.wantTypeVal {
				t.Errorf("expected type field '%s', got '%s'", tc.wantTypeVal, att.Fields[0].Value)
			}
		})
	}
}

func TestResolveDisplayName(t *testing.T) {
	tests := []struct {
		displayName string
		fallback    string
		want        string
	}{
		{"Alice", "alice", "Alice"},
		{"", "bob", "bob"},
		{"", "", "N/A"},
		{"Charlie", "", "Charlie"},
	}

	for _, tc := range tests {
		got := resolveDisplayName(tc.displayName, tc.fallback)
		if got != tc.want {
			t.Errorf("resolveDisplayName(%q, %q) = %q, want %q", tc.displayName, tc.fallback, got, tc.want)
		}
	}
}
