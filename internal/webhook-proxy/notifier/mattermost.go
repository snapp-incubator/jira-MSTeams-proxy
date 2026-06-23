package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
)

// MattermostNotifier implements Notifier for Mattermost via Slack-compatible attachments.
type MattermostNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewMattermostNotifier creates a new Mattermost notifier.
func NewMattermostNotifier(webhookURL string) *MattermostNotifier {
	return &MattermostNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the channel identifier.
func (n *MattermostNotifier) Name() string {
	return "mattermost"
}

// Send dispatches a Jira notification to Mattermost.
func (n *MattermostNotifier) Send(ctx context.Context, req *request.JiraRequest, isComment bool, team string) error {
	logrus.Printf("[mattermost] Team: %s, Key: %s, Summary: %s", team, req.Key, req.Fields.Summary)

	payload := n.buildPayload(req, isComment)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("[mattermost] marshal request body error: %w", err)
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("[mattermost] create request error: %w", err)
	}
	r.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := n.client.Do(r)
	if err != nil {
		return fmt.Errorf("[mattermost] request error: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Errorf("[mattermost] error closing response body: %s", err)
		}
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logrus.Infof("[mattermost] successfully sent webhook for %s", req.Key)
		return nil
	}

	responseBodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("[mattermost] failed with status %d (response body read error: %w)", resp.StatusCode, readErr)
	}
	return fmt.Errorf("[mattermost] failed with status %d, response: %s", resp.StatusCode, string(responseBodyBytes))
}

// buildPayload constructs a MattermostPayload with Slack-compatible attachments.
func (n *MattermostNotifier) buildPayload(req *request.JiraRequest, isComment bool) MattermostPayload {
	// Resolve display names
	creatorDisplayName := resolveDisplayName(req.Fields.Creator.DisplayName, req.Fields.Creator.Name)
	assigneeDisplayName := resolveDisplayName(req.Fields.Assignee.DisplayName, req.Fields.Assignee.Name)

	// Resolve type and link
	requestTypeName := "N/A"
	if req.Fields.CustomField10003.RequestType.Name != "" {
		requestTypeName = req.Fields.CustomField10003.RequestType.Name
	}

	webLink := ""
	if req.Fields.CustomField10003.Links.Web != "" {
		webLink = req.Fields.CustomField10003.Links.Web
	}

	summary := "N/A"
	if req.Fields.Summary != "" {
		summary = req.Fields.Summary
	}

	// Choose color: blue for comments, green for issues
	color := "#36a64f"
	title := fmt.Sprintf("🎯 %s", req.Key)
	fallback := fmt.Sprintf("New Issue/Update: %s - %s", req.Key, summary)
	if isComment {
		color = "#2196F3"
		title = fmt.Sprintf("📰 New Comment on %s", req.Key)
		fallback = fmt.Sprintf("New Comment on %s: %s", req.Key, summary)
	}

	attachment := MattermostAttachment{
		Fallback:  fallback,
		Color:     color,
		Title:     title,
		TitleLink: webLink,
		Fields: []MattermostField{
			{Title: "Type", Value: requestTypeName, Short: true},
			{Title: "Summary", Value: summary, Short: false},
			{Title: "Issuer", Value: creatorDisplayName, Short: true},
			{Title: "Assignee", Value: assigneeDisplayName, Short: true},
		},
	}

	return MattermostPayload{
		Attachments: []MattermostAttachment{attachment},
	}
}

// resolveDisplayName returns the best available display name.
func resolveDisplayName(displayName, fallbackName string) string {
	if displayName != "" {
		return displayName
	}
	if fallbackName != "" {
		return fallbackName
	}
	return "N/A"
}
