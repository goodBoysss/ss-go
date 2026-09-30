// Command ssgo runs the encrypted relay server or local SOCKS5 client.
package main

import (
	"flag"
	"log"

	"shadowsocks-go/internal/app"
	"shadowsocks-go/internal/config"
)

// main loads configuration and starts the selected program mode.
func main() {
	configPath := flag.String("config", "config.json", "path to JSON configuration")
	modeOverride := flag.String("mode", "", "optional mode override: server or client")
	flag.Parse()

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
