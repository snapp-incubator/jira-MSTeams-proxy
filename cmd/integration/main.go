package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/config"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/handler"
	"github.com/snapp-incubator/jira-msteams-proxy/internal/webhook-proxy/notifier"
)

func main() {
	// Mock receivers
	msteamsMock := startMockReceiver("msteams", ":9001")
	_ = msteamsMock

	// MSTeams → local mock; Mattermost → real Snapp webhook
	msteamsConf := config.MSTeamsConfig{
		URL:         "http://localhost:9001/",
		RuntimeURL:  "http://localhost:9001/runtime",
		PlatformURL: "http://localhost:9001/platform",
		NetworkURL:  "http://localhost:9001/network",
	}

	mattermostWebhook := "https://chat..."

	var notifiers []notifier.Notifier
	notifiers = append(notifiers, notifier.NewMSTeamsNotifier(msteamsConf))
	notifiers = append(notifiers, notifier.NewMattermostNotifier(mattermostWebhook))

	proxyHandler := handler.Proxy{Notifiers: notifiers}


	app := echo.New()
	app.HideBanner = true

	app.GET("/healthz", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	app.POST("/:team", proxyHandler.HandleJiraWebhook(false))
	app.POST("/comment/:team", proxyHandler.HandleJiraWebhook(true))
	app.POST("/", proxyHandler.HandleJiraWebhook(false))
	app.POST("/comment", proxyHandler.HandleJiraWebhook(true))


	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  LOCAL INTEGRATION TEST — all services running                     ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Proxy:       http://localhost:8080                                ║")
	fmt.Println("║  MSTeams:     http://localhost:9001  (local mock)                  ║")
	fmt.Println("║  Mattermost:  https://chat.snapp.services/hooks/... (real)        ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  TEST COMMANDS (copy-paste into another terminal):                 ║")
	fmt.Println("║                                                                    ║")
	fmt.Println("║  # Issue to default team                                           ║")
	fmt.Println(`║  curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/ \║`)
	fmt.Println(`║    -H "Content-Type: application/json" -d @sample_jira_payload.json   ║`)
	fmt.Println("║                                                                    ║")
	fmt.Println("║  # Issue to platform team                                          ║")
	fmt.Println(`║  curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/platform \║`)
	fmt.Println(`║    -H "Content-Type: application/json" -d @sample_jira_payload.json   ║`)
	fmt.Println("║                                                                    ║")
	fmt.Println("║  # Comment to platform team                                        ║")
	fmt.Println(`║  curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/comment/platform \║`)
	fmt.Println(`║    -H "Content-Type: application/json" -d @sample_jira_payload.json   ║`)
	fmt.Println("║                                                                    ║")
	fmt.Println("║  # Invalid body (expect 400)                                       ║")
	fmt.Println(`║  curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/platform \║`)
	fmt.Println(`║    -H "Content-Type: application/json" -d '{bad json'                 ║`)
	fmt.Println("║                                                                    ║")
	fmt.Println("║  Expected: proxy returns 200, mock servers print received payloads ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()


	go func() {
		if err := app.Start(":8080"); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("proxy start failed: %s", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nShutting down...")
}

func startMockReceiver(name, addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		separator := strings.Repeat("─", 60)
		fmt.Printf("\n%s\n📩 [%s] %s %s\n", separator, name, r.Method, r.URL.Path)
		fmt.Printf("   Content-Type: %s\n", r.Header.Get("Content-Type"))
		if len(body) > 0 {
			fmt.Printf("   Body (%d bytes):\n%s\n", len(body), string(body))
		}
		fmt.Println(separator)

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		logrus.Printf("Mock [%s] listening on %s", name, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("mock [%s] failed: %s", name, err)
		}
	}()
	return srv
}
