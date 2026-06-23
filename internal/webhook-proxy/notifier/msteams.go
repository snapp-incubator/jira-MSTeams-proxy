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
	"github.com/snapp-incubator/jira-msteams-proxy/internal/config"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
)

const (
	platformSubteam = "platform"
	networkSubteam  = "network"
	runtimeSubteam  = "runtime"
)

// MSTeamsNotifier implements Notifier for Microsoft Teams.
type MSTeamsNotifier struct {
	conf   config.MSTeamsConfig
	client *http.Client
}

// NewMSTeamsNotifier creates a new MSTeams notifier.
func NewMSTeamsNotifier(conf config.MSTeamsConfig) *MSTeamsNotifier {
	return &MSTeamsNotifier{
		conf: conf,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the channel identifier.
func (n *MSTeamsNotifier) Name() string {
	return "msteams"
}

// Send dispatches a Jira notification to Microsoft Teams.
func (n *MSTeamsNotifier) Send(ctx context.Context, req *request.JiraRequest, isComment bool, team string) error {
	generatedCard := generateTeamsAdaptiveCard(req, isComment)
	logrus.Printf("[msteams] Team: %s, Generated Text: %+v", team, generatedCard)

	targetURL := n.resolveTeamURL(team)

	if targetURL == "" {
		logrus.Warnf("[msteams] No MS Teams URL configured for team '%s' (or default). Skipping notification.", team)
		return nil
	}

	return n.sendCard(ctx, generatedCard, targetURL)
}

// resolveTeamURL picks the correct MS Teams webhook URL for the given team.
func (n *MSTeamsNotifier) resolveTeamURL(team string) string {
	switch team {
	case platformSubteam:
		logrus.Printf("[msteams] Using platform url for team '%s'", team)
		return n.conf.PlatformURL
	case networkSubteam:
		logrus.Printf("[msteams] Using network url for team '%s'", team)
		return n.conf.NetworkURL
	case runtimeSubteam:
		logrus.Printf("[msteams] Using runtime url for team '%s'", team)
		return n.conf.RuntimeURL
	default:
		logrus.Printf("[msteams] Using default url for team '%s'", team)
		return n.conf.URL
	}
}

// sendCard marshals and POSTs the Adaptive Card to the Teams webhook URL.
func (n *MSTeamsNotifier) sendCard(ctx context.Context, card adaptiveCard, webhookURL string) error {
	payload := mSTeamsAdaptiveCardMessage{
		Type: "message",
		Attachments: []attachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content:     card,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("[msteams] marshal request body error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("[msteams] create request error for URL %s: %w", webhookURL, err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("[msteams] request error for URL %s: %w", webhookURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Errorf("[msteams] error closing response body: %s", err)
		}
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logrus.Infof("[msteams] successfully sent webhook to %s", webhookURL)
		return nil
	}

	responseBodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("[msteams] failed with status %d from %s (response body read error: %w)", resp.StatusCode, webhookURL, readErr)
	}
	return fmt.Errorf("[msteams] failed with status %d from %s, response: %s", resp.StatusCode, webhookURL, string(responseBodyBytes))
}

// generateTeamsAdaptiveCard builds an MSTeams Adaptive Card from a Jira request.
func generateTeamsAdaptiveCard(req *request.JiraRequest, isComment bool) adaptiveCard {
	creatorDisplayName := "N/A"
	creatorMentionID := ""
	if req.Fields.Creator.EmailAddress != "" {
		creatorMentionID = req.Fields.Creator.EmailAddress
		if req.Fields.Creator.DisplayName != "" {
			creatorDisplayName = req.Fields.Creator.DisplayName
		} else {
			creatorDisplayName = req.Fields.Creator.Name
		}
	} else if req.Fields.Creator.DisplayName != "" {
		creatorDisplayName = req.Fields.Creator.DisplayName
	}

	assigneeDisplayName := "N/A"
	assigneeMentionID := ""
	if req.Fields.Assignee.EmailAddress != "" {
		assigneeMentionID = req.Fields.Assignee.EmailAddress
		if req.Fields.Assignee.DisplayName != "" {
			assigneeDisplayName = req.Fields.Assignee.DisplayName
		} else {
			assigneeDisplayName = req.Fields.Assignee.Name
		}
	} else if req.Fields.Assignee.DisplayName != "" {
		assigneeDisplayName = req.Fields.Assignee.DisplayName
	}

	requestTypeName := "N/A"
	webLink := "#"
	if req.Fields.CustomField10003.RequestType.Name != "" {
		requestTypeName = req.Fields.CustomField10003.RequestType.Name
	}
	if req.Fields.CustomField10003.Links.Web != "" {
		webLink = req.Fields.CustomField10003.Links.Web
	}
	summary := "N/A"
	if req.Fields.Summary != "" {
		summary = req.Fields.Summary
	}
	var title string
	if isComment {
		title = "📰 **New Comment Added**"
	} else {
		title = "🎯 **New Issue/Update**"
	}

	mentionEntities := []mentionEntity{}

	creatorMentionText := creatorDisplayName
	if creatorMentionID != "" {
		creatorMentionText = fmt.Sprintf("<at>%s</at>", creatorDisplayName)
		mentionEntities = append(mentionEntities, mentionEntity{
			Type: "mention",
			Text: creatorMentionText,
			Mentioned: mentionedUser{
				ID:   creatorMentionID,
				Name: creatorDisplayName,
			},
		})
	}

	assigneeMentionText := assigneeDisplayName
	if assigneeMentionID != "" {
		assigneeMentionText = fmt.Sprintf("<at>%s</at>", assigneeDisplayName)
		mentionEntities = append(mentionEntities, mentionEntity{
			Type: "mention",
			Text: assigneeMentionText,
			Mentioned: mentionedUser{
				ID:   assigneeMentionID,
				Name: assigneeDisplayName,
			},
		})
	}

	cardBody := []interface{}{
		textBlock{Type: "TextBlock", Text: title, Weight: "bolder", Size: "medium", Wrap: true},
		factSet{Type: "FactSet", Facts: []fact{
			{Title: "Type:", Value: requestTypeName},
			{Title: "Summary:", Value: summary},
			{Title: "Issuer:", Value: creatorMentionText},
			{Title: "Assignee:", Value: assigneeMentionText},
		}},
	}

	card := adaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.5",
		Body:    cardBody,
		Actions: []interface{}{
			actionOpenURL{Type: "Action.OpenUrl", Title: "View Issue in Jira", URL: webLink},
		},
	}

	if len(mentionEntities) > 0 {
		card.MSTeams = &mSTeamsInfo{
			Entities: mentionEntities,
		}
	}

	return card
}
