package app

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"shadowsocks-go/internal/config"
)

// RunServer starts one standard Shadowsocks listener for each configured account.
func RunServer(cfg *config.Config) error {
	listeners := make([]net.Listener, 0, len(cfg.Users))
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	ciphers := make([]core.Cipher, 0, len(cfg.Users))
	for _, user := range cfg.Users {
		cipher, err := core.PickCipher(user.Method, nil, user.Password)
		if err != nil {
			return fmt.Errorf("create cipher for %s: %w", user.Listen, err)
		}
		listener, err := net.Listen("tcp", user.Listen)
		if err != nil {
			return fmt.Errorf("listen Shadowsocks on %s: %w", user.Listen, err)
		}
		listeners = append(listeners, listener)
		ciphers = append(ciphers, cipher)
	}
	errors := make(chan error, len(listeners))
	for index, listener := range listeners {
		log.Printf("Shadowsocks server listening on %s with %s", listener.Addr(), cfg.Users[index].Method)
		go func(listener net.Listener, cipher core.Cipher) {
			errors <- acceptServerConnections(listener, cipher, cfg.Server.DialTimeoutSeconds)
		}(listener, ciphers[index])
	}
	return <-errors
}

// acceptServerConnections accepts encrypted clients for one configured listener.
func acceptServerConnections(listener net.Listener, cipher core.Cipher, dialTimeoutSeconds int) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accept Shadowsocks connection: %w", err)
		}
		go serveServerConnection(wrapCipher(conn, cipher), dialTimeoutSeconds)
	}
}

// serveServerConnection reads a standard Shadowsocks target and relays TCP data.
func serveServerConnection(conn net.Conn, dialTimeoutSeconds int) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	target, err := socks.ReadAddr(conn)
	if err != nil {
		log.Printf("read Shadowsocks target from %s failed: %v", conn.RemoteAddr(), err)
		return
	}
	_ = conn.SetDeadline(time.Time{})
	dialTimeout := 10 * time.Second
	if dialTimeoutSeconds > 0 {
		dialTimeout = time.Duration(dialTimeoutSeconds) * time.Second
	}
	remote, err := net.DialTimeout("tcp", target.String(), dialTimeout)
	if err != nil {
		log.Printf("connect target %s failed: %v", target, err)
		return
	}
	defer remote.Close()
	log.Printf("Shadowsocks %s connected to %s", conn.RemoteAddr(), target)
	relay(conn, remote)
}
