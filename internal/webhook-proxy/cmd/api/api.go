package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/snapp-incubator/jira-msteams-proxy/internal/config"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/handler"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/notifier"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main(cfg config.Config) {
	app := echo.New()

	var notifiers []notifier.Notifier
	notifiers = append(notifiers, notifier.NewMSTeamsNotifier(cfg.MSTeams))
	if cfg.Mattermost.Webhook != "" {
		notifiers = append(notifiers, notifier.NewMattermostNotifier(cfg.Mattermost.Webhook))
		logrus.Println("Mattermost notifier enabled")
	}

	proxyHandler := handler.Proxy{Notifiers: notifiers}

	logrus.Printf("API started with %d notifier(s) :D", len(notifiers))

	app.GET("/healthz", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	app.POST("/:team", proxyHandler.HandleJiraWebhook(false))
	app.POST("/comment/:team", proxyHandler.HandleJiraWebhook(true))
	app.POST("/", proxyHandler.HandleJiraWebhook(false))
	app.POST("/comment", proxyHandler.HandleJiraWebhook(true))

	if err := app.Start(fmt.Sprintf(":%d", cfg.API.Port)); !errors.Is(err, http.ErrServerClosed) {
		logrus.Fatalf("echo initiation failed: %s", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func Register(root *cobra.Command, cfg config.Config) {
	root.AddCommand(
		&cobra.Command{
			Use:   "api",
			Short: "Run API to serve the requests",
			Run: func(cmd *cobra.Command, args []string) {
				main(cfg)
			},
		},
	)
}
