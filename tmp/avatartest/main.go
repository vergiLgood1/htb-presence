// Command avatartest is a throwaway visual check for external-image assets.
// It is not part of the release and is deleted after use.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
)

func main() {
	clientID := os.Getenv("CLIENT_ID")
	if clientID == "" || len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: CLIENT_ID=... avatartest <image-url>")
		os.Exit(2)
	}

	ctx := context.Background()
	client, err := discord.Dial(ctx, clientID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	defer client.Close()

	activity := &discord.Activity{
		Details:    "Avatar test",
		State:      "external URL via media proxy",
		LargeImage: os.Args[1],
		LargeText:  "Vaccine",
		SmallImage: "htb",
		SmallText:  "Hack The Box",
	}
	if err := client.SetActivity(ctx, activity); err != nil {
		fmt.Fprintln(os.Stderr, "set activity:", err)
		os.Exit(1)
	}
	fmt.Println("activity set — look at your Discord profile for ~2min")
	time.Sleep(120 * time.Second)
	client.SetActivity(ctx, nil)
	fmt.Println("cleared")
}
