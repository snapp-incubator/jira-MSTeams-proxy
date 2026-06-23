package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/notifier"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
)

// mockNotifier is a test double that records calls and optionally fails.
type mockNotifier struct {
	name      string
	callCount int32
	shouldErr bool
}

var _ notifier.Notifier = (*mockNotifier)(nil)

func (m *mockNotifier) Name() string { return m.name }

func (m *mockNotifier) Send(_ context.Context, _ *request.JiraRequest, _ bool, _ string) error {
	atomic.AddInt32(&m.callCount, 1)
	if m.shouldErr {
		return errors.New("mock send error")
	}
	return nil
}

func (m *mockNotifier) CallCount() int32 {
	return atomic.LoadInt32(&m.callCount)
}

func TestHandleJiraWebhook_FanOut(t *testing.T) {
	n1 := &mockNotifier{name: "notifier1"}
	n2 := &mockNotifier{name: "notifier2"}

	proxy := &Proxy{Notifiers: []notifier.Notifier{n1, n2}}

	body := sampleJSON()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/platform", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:team")
	c.SetParamNames("team")
	c.SetParamValues("platform")

	handler := proxy.HandleJiraWebhook(false)
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %s", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if n1.CallCount() != 1 {
		t.Errorf("expected notifier1 called once, got %d", n1.CallCount())
	}
	if n2.CallCount() != 1 {
		t.Errorf("expected notifier2 called once, got %d", n2.CallCount())
	}
}

func TestHandleJiraWebhook_OneNotifierFails(t *testing.T) {
	n1 := &mockNotifier{name: "success", shouldErr: false}
	n2 := &mockNotifier{name: "failure", shouldErr: true}

	proxy := &Proxy{Notifiers: []notifier.Notifier{n1, n2}}

	body := sampleJSON()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/platform", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:team")
	c.SetParamNames("team")
	c.SetParamValues("platform")

	handler := proxy.HandleJiraWebhook(false)
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %s", err)
	}

	// Must still return 200 OK even when one notifier fails
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 even on notifier failure, got %d", rec.Code)
	}
	// Both notifiers should have been called
	if n1.CallCount() != 1 {
		t.Errorf("expected success notifier called once, got %d", n1.CallCount())
	}
	if n2.CallCount() != 1 {
		t.Errorf("expected failure notifier called once, got %d", n2.CallCount())
	}
}

func TestHandleJiraWebhook_InvalidBody(t *testing.T) {
	n1 := &mockNotifier{name: "notifier1"}
	proxy := &Proxy{Notifiers: []notifier.Notifier{n1}}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/platform", strings.NewReader("{invalid json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:team")
	c.SetParamNames("team")
	c.SetParamValues("platform")

	handler := proxy.HandleJiraWebhook(false)
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %s", err)
	}

	// Should return 400 for invalid body
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid body, got %d", rec.Code)
	}
	// Notifier should NOT have been called
	if n1.CallCount() != 0 {
		t.Errorf("expected notifier not called for invalid body, got %d calls", n1.CallCount())
	}
}

func TestHandleJiraWebhook_EmptyNotifierList(t *testing.T) {
	proxy := &Proxy{Notifiers: nil}

	body := sampleJSON()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/platform", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:team")
	c.SetParamNames("team")
	c.SetParamValues("platform")

	handler := proxy.HandleJiraWebhook(false)
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %s", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 with no notifiers, got %d", rec.Code)
	}
}

// sampleJSON returns a minimal valid Jira webhook JSON body.
func sampleJSON() string {
	req := request.JiraRequest{
		Key: "PROJ-125",
		Fields: request.Fields{
			Summary: "Test issue",
			Creator: request.User{
				DisplayName: "Test User",
				Name:        "testuser",
			},
			Assignee: request.User{
				DisplayName: "Assignee User",
				Name:        "assignee",
			},
			CustomField10003: request.CustomField10003{
				RequestType: request.RequestType{Name: "Bug Report"},
				Links:       request.Links{Web: "http://jira.example.com/PROJ-125"},
			},
		},
	}
	b, _ := json.Marshal(req)
	return string(b)
}
