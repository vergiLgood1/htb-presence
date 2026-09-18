// Command htb-presence shows the current Hack The Box activity of the configured
// user as Discord Rich Presence.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/config"
	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/history"
	"github.com/vergiLgood1/htb-presence/internal/htb"
	"github.com/vergiLgood1/htb-presence/internal/presence"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	defaultPath, err := config.DefaultPath()
	if err != nil {
		return err
	}

	configPath := flag.String("config", defaultPath, "path to the config file")
	once := flag.Bool("once", false, "fetch the current activity once, print it, and exit (debug)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if *showVersion {
		fmt.Printf("htb-presence %s\n", version)
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	if *once {
		logConfig(*configPath, cfg)
		return printOnce(cfg)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Watch the config file and restart the scheduler when it changes. An
	// invalid edit is ignored (and retried) until it becomes valid.
	reload := make(chan *config.Config, 1)
	go config.Watcher{Path: *configPath}.Watch(ctx, func() error {
		next, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		select {
		case reload <- next:
		default:
		}
		return nil
	})

	for {
		logConfig(*configPath, cfg)

		recorder, err := openHistory(cfg)
		if err != nil {
			return err
		}

		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			newScheduler(cfg, recorder).Run(runCtx)
			close(done)
		}()

		select {
		case <-ctx.Done():
			cancel()
			<-done
			recorder.Close()
			return nil
		case next := <-reload:
			slog.Info("config changed, restarting with the new settings")
			cfg = next
			cancel()
			<-done
			recorder.Close()
		}
	}
}

// openHistory opens the optional session-history file, or nil when disabled.
func openHistory(cfg *config.Config) (*history.Recorder, error) {
	if cfg.History.File == "" {
		return nil, nil
	}
	return history.Open(cfg.History.File)
}

// newScheduler wires the HTB client, Discord IPC connection and poll loop from a
// resolved config.
func newScheduler(cfg *config.Config, recorder *history.Recorder) *presence.Scheduler {
	scheduler := &presence.Scheduler{
		Fetcher: htb.NewClient(cfg.HTB.APIToken),
		Connect: func(ctx context.Context) (presence.DiscordClient, error) {
			return discord.Dial(ctx, cfg.Discord.ClientID)
		},
		Interval: time.Duration(cfg.HTB.PollInterval),
		Options: presence.Options{
			ShowMachineName: cfg.Discord.ShowMachineName,
			ShowRank:        cfg.Discord.ShowRank,
			ShowTimer:       cfg.Discord.ShowTimer,
		},
		Logger: slog.Default(),
	}
	if recorder != nil {
		scheduler.History = recorder
	}
	return scheduler
}

func logConfig(path string, cfg *config.Config) {
	slog.Info("config loaded",
		"version", version,
		"path", path,
		"poll_interval", time.Duration(cfg.HTB.PollInterval),
		"htb_token", config.MaskToken(cfg.HTB.APIToken),
	)
}

// printOnce fetches the current activity once and logs it, without touching
// Discord. It is a credential/endpoint smoke test.
func printOnce(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	activity, err := htb.NewClient(cfg.HTB.APIToken).CurrentActivity(ctx)
	if err != nil {
		return fmt.Errorf("fetching HTB activity: %w", err)
	}
	if activity.Machine == nil {
		slog.Info("no active machine")
		return nil
	}

	m := activity.Machine
	slog.Info("active machine",
		"id", m.ID,
		"name", m.Name,
		"os", m.OS,
		"difficulty", m.Difficulty,
		"ip", m.IP,
		"expires_at", m.ExpiresAt,
	)
	return nil
}
