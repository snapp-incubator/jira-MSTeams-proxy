package handler

import (
	"bytes"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/notifier"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/request"
	"golang.org/x/sync/errgroup"
)

// Proxy fans out Jira webhooks to all registered Notifiers concurrently.
type Proxy struct {
	Notifiers []notifier.Notifier
}

// HandleJiraWebhook returns an Echo handler that binds a Jira webhook body
// and dispatches it to every registered Notifier in parallel.
// HTTP response to Jira is always 200 OK (errors are logged, not propagated).
func (p *Proxy) HandleJiraWebhook(isComment bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		team := c.Param("team")
		ctx := c.Request().Context()

		req := &request.JiraRequest{}

		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			logrus.Errorf("Failed to read request body: %s", err.Error())
			return c.String(http.StatusInternalServerError, "Error reading body")
		}
		c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		err = c.Bind(req)
		if err != nil {
			logrus.Errorf("failed to bind request body: %s", err.Error())
			logrus.Debugf("Problematic request body for bind: %s", string(bodyBytes))
			return c.NoContent(http.StatusBadRequest)
		}

		logrus.Printf("Team: %s, Key: %s, Summary: %s", team, req.Key, req.Fields.Summary)

		var g errgroup.Group
		for _, n := range p.Notifiers {
			notifierRef := n
			g.Go(func() error {
				if err := notifierRef.Send(ctx, req, isComment, team); err != nil {
					logrus.Errorf("[%s] failed to send notification for team '%s': %s", notifierRef.Name(), team, err)
					return err
				}
				return nil
			})
		}

		// Wait for all notifiers; errors are logged inside the goroutines above.
		_ = g.Wait()

		// Always return 200 OK to Jira — errors are logged, not propagated.
		return c.NoContent(http.StatusOK)
	}
}
