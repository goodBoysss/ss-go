package app

import (
	"io"
	"net"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
)

// encryptedConn preserves TCP half-close support around an encrypted stream.
type encryptedConn struct {
	net.Conn
	raw net.Conn
}

// wrapCipher wraps an accepted TCP connection with the standard Shadowsocks cipher.
func wrapCipher(raw net.Conn, cipher core.StreamConnCipher) net.Conn {
	return &encryptedConn{Conn: cipher.StreamConn(raw), raw: raw}
}

// CloseWrite sends TCP FIN after encrypted output has been flushed.
func (c *encryptedConn) CloseWrite() error {
	return closeWrite(c.raw)
}

// closeWrite closes the send direction without discarding pending responses.
func closeWrite(conn net.Conn) error {
	if tcp, ok := conn.(interface{ CloseWrite() error }); ok {
		return tcp.CloseWrite()
	}
	return conn.Close()
}

// relay copies both directions, preserving half-close and bounding response drain time.
func relay(left, right net.Conn) {
	done := make(chan struct{}, 2)
	copyDirection := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		if err != nil {
			_ = left.Close()
			_ = right.Close()
		} else {
			_ = closeWrite(dst)
			_ = dst.SetReadDeadline(time.Now().Add(30 * time.Second))
		}
		done <- struct{}{}
	}
	go copyDirection(left, right)
	go copyDirection(right, left)
	<-done
	<-done
}
