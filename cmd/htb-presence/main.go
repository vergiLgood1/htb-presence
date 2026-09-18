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
	"github.com/vergiLgood1/htb-presence/internal/htb"
	"github.com/vergiLgood1/htb-presence/internal/presence"
)

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
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	slog.Info("config loaded",
		"path", *configPath,
		"poll_interval", time.Duration(cfg.HTB.PollInterval),
		"htb_token", config.MaskToken(cfg.HTB.APIToken),
	)

	if *once {
		return printOnce(cfg)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	scheduler.Run(ctx)
	return nil
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
