package notifier

import (
	"context"

	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
)

// Notifier is the abstraction for a notification channel (MSTeams, Mattermost, etc.).
type Notifier interface {
	// Name returns the channel identifier for logging (e.g. "msteams", "mattermost").
	Name() string

	// Send dispatches a Jira notification to the channel.
	Send(ctx context.Context, req *request.JiraRequest, isComment bool, team string) error
}
