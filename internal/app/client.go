package app

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"shadowsocks-go/internal/config"
	"shadowsocks-go/internal/socks5"
)

// RunClient starts a local SOCKS5 listener for a standard Shadowsocks server.
func RunClient(cfg *config.Config) error {
	listener, err := net.Listen("tcp", cfg.Client.Listen)
	if err != nil {
		return fmt.Errorf("listen client on %s: %w", cfg.Client.Listen, err)
	}
	defer listener.Close()
	cipher, err := pickCipher(cfg.Client.Method, cfg.Client.Password)
	if err != nil {
		return fmt.Errorf("create client cipher: %w", err)
	}
	log.Printf("SOCKS5 listening on %s, Shadowsocks server %s with %s", cfg.Client.Listen, cfg.Client.Server, cfg.Client.Method)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accept SOCKS5 connection: %w", err)
		}
		go serveClientConnection(conn, cfg.Client.Server, cipher)
	}
}

// serveClientConnection forwards one local SOCKS5 CONNECT through Shadowsocks.
func serveClientConnection(local net.Conn, serverAddress string, cipher core.StreamConnCipher) {
	defer local.Close()
	_ = local.SetDeadline(time.Now().Add(15 * time.Second))
	target, err := socks5.Accept(local)
	if err != nil {
		log.Printf("SOCKS5 request from %s failed: %v", local.RemoteAddr(), err)
		return
	}
	raw, err := net.DialTimeout("tcp", serverAddress, 10*time.Second)
	if err != nil {
		_ = socks5.ReplyFailure(local)
		log.Printf("connect Shadowsocks server %s failed: %v", serverAddress, err)
		return
	}
	remote := wrapCipher(raw, cipher)
	defer remote.Close()
	_ = remote.SetDeadline(time.Now().Add(15 * time.Second))
	address := socks.ParseAddr(target)
	if address == nil {
		_ = socks5.ReplyFailure(local)
		return
	}
	if _, err := remote.Write(address); err != nil {
		_ = socks5.ReplyFailure(local)
		return
	}
	if err := socks5.ReplySuccess(local); err != nil {
		return
	}
	_ = local.SetDeadline(time.Time{})
	_ = remote.SetDeadline(time.Time{})
	log.Printf("SOCKS5 %s connected to %s", local.RemoteAddr(), target)
	relay(local, remote)
}
