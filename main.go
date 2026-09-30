// Command ssgo runs the encrypted relay server or local SOCKS5 client.
package main

import (
	"flag"
	"log"

	"shadowsocks-go/internal/app"
	"shadowsocks-go/internal/config"
	"shadowsocks-go/internal/logging"
)

// main initializes daily logs, loads configuration, and starts the selected program mode.
func main() {
	configPath := flag.String("config", "config.json", "path to JSON configuration")
	modeOverride := flag.String("mode", "", "optional mode override: server or client")
	flag.Parse()

	writer, err := logging.NewDailyWriter("log")
	if err != nil {
		log.Fatal(err)
	}
	defer writer.Close()
	log.SetOutput(writer)

	cfg, err := config.Load(*configPath, *modeOverride)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.Mode == "server" {
		log.Fatal(app.RunServer(cfg))
	}
	if cfg.Mode == "client" {
		log.Fatal(app.RunClient(cfg))
	}
	log.Fatalf("unsupported mode %q", cfg.Mode)
}
