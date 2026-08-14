// Package tray implements the GoModel menu-bar app: it starts the gateway in
// the background, shows a status-bar icon whose menu reports service health,
// opens the dashboard in the default browser, reloads configuration, and
// quits the gateway on demand. The macOS release is packaged as GoModel.app
// (see `make app`); the same binary also runs on Windows and Linux trays.
package tray

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fyne.io/systray"

	"github.com/enterpilot/gomodel/config"
	"github.com/enterpilot/gomodel/internal/platformdir"
	"github.com/enterpilot/gomodel/internal/version"
	"github.com/enterpilot/gomodel/run"
)

//go:embed assets/icon.png
var menuIcon []byte

const (
	statusPollInterval = 3 * time.Second
	healthTimeout      = 2 * time.Second
	gatewayWaitTimeout = 35 * time.Second // run's shutdown budget plus margin
)

// Run is the menu-bar app entry point. It must be called from the main
// goroutine: systray owns the macOS/Windows event loop there, and the gateway
// runs on a background goroutine. SIGINT/SIGTERM — and the Quit menu item —
// shut the gateway down and exit the whole app.
func Run() error {
	migrateLegacyData()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	gatewayErr := make(chan error, 1)
	go func() {
		gatewayErr <- run.Run(ctx, run.Options{ProductName: "gomodel", Stderr: logWriter()})
	}()

	// A signal (or the Quit menu, which cancels ctx) ends the event loop so
	// Run can return once the gateway has shut down.
	go func() {
		<-ctx.Done()
		systray.Quit()
	}()

	systray.Run(
		func() { onReady(ctx, cancel, gatewayErr) },
		func() { cancel() },
	)

	select {
	case <-gatewayErr:
	case <-time.After(gatewayWaitTimeout):
	}
	return nil
}

// logWriter returns a log file inside the per-user data directory so menu-bar
// runs (which have no terminal) keep a record, falling back to os.Stderr.
func logWriter() *os.File {
	path := platformdir.DataFile("gomodel.log")
	if path == "" {
		return os.Stderr
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return os.Stderr
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		return f
	}
	return os.Stderr
}

// service describes the gateway's address; resolved once at startup and
// shared by the health poller and the dashboard opener.
type service struct {
	port     string
	basePath string
	url      *url.URL
}

func resolveService() service {
	s := service{port: "8080", basePath: "/"}
	if result, err := config.Load(); err == nil {
		if p := strings.TrimSpace(result.Config.Server.Port); p != "" {
			s.port = p
		}
		s.basePath = result.Config.Server.BasePath
	}
	s.url = &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", s.port),
		Path:   config.JoinBasePath(s.basePath, "/admin/dashboard"),
	}
	return s
}

func onReady(ctx context.Context, cancel context.CancelFunc, gatewayErr <-chan error) {
	svc := resolveService()

	systray.SetIcon(menuIcon)
	// No SetTitle: the menu bar shows the icon only, without a text label.
	systray.SetTooltip("GoModel — starting…")

	statusItem := systray.AddMenuItem("Status: Starting…", "GoModel gateway service status")
	statusItem.Disable()
	systray.AddSeparator()
	openItem := systray.AddMenuItem("Open Dashboard", "Open the GoModel dashboard in your browser")
	reloadItem := systray.AddMenuItem("Reload Configuration", "Re-read .env and config.yaml")
	if runtime.GOOS == "windows" {
		reloadItem.Hide()
	}
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Quit GoModel")

	ticker := time.NewTicker(statusPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-quitItem.ClickedCh:
			// The ctx watcher in Run quits systray once the gateway exits.
			cancel()
			return
		case <-openItem.ClickedCh:
			openInBrowser(svc.url.String())
		case <-reloadItem.ClickedCh:
			reloadGateway()
		case <-ticker.C:
			updateStatus(statusItem, svc, gatewayErr)
		}
	}
}

// updateStatus reflects the gateway's health in the menu label and tooltip.
func updateStatus(item *systray.MenuItem, svc service, gatewayErr <-chan error) {
	select {
	case err := <-gatewayErr:
		label := "Status: Stopped"
		if err != nil {
			label = "Status: Error — " + conciseError(err)
		}
		item.SetTitle(label)
		systray.SetTooltip("GoModel — " + label)
		return
	default:
	}

	if err := checkHealth(svc); err != nil {
		item.SetTitle("Status: Starting…")
		systray.SetTooltip("GoModel — starting…")
		return
	}
	item.SetTitle("Status: Running")
	systray.SetTooltip(fmt.Sprintf("GoModel %s — running on %s", version.Version, svc.url.Host))
}

func checkHealth(svc service) error {
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", svc.port),
		Path:   config.JoinBasePath(svc.basePath, "/health"),
	}
	client := &http.Client{Timeout: healthTimeout}
	resp, err := client.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func reloadGateway() {
	if runtime.GOOS == "windows" {
		return
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		slog.Error("failed to send reload signal", "error", err)
	}
}

func openInBrowser(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		slog.Error("failed to open the dashboard", "url", target, "error", err)
	}
}

func conciseError(err error) string {
	if err == nil {
		return "unknown"
	}
	// A bind failure names the address; keep the message short for the menu.
	msg := err.Error()
	if i := strings.Index(msg, ": "); i > 0 && len(msg) > 40 {
		return msg[:i]
	}
	return msg
}
