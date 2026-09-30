package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/shadowsocks/go-shadowsocks2/core"
)

// pickCipher selects standard AEAD or the explicit legacy AES-256-CFB compatibility mode.
func pickCipher(method, password string) (core.StreamConnCipher, error) {
	if method != "aes-256-cfb" {
		return core.PickCipher(method, nil, password)
	}
	if password == "" {
		return nil, errors.New("password is required")
	}
	block, err := aes.NewCipher(legacyKey(password))
	if err != nil {
		return nil, err
	}
	return &cfbCipher{block: block}, nil
}

// legacyKey implements Shadowsocks' historical EVP_BytesToKey MD5 derivation.
func legacyKey(password string) []byte {
	key := make([]byte, 0, 32)
	var previous []byte
	for len(key) < 32 {
		hash := md5.New()
		_, _ = hash.Write(previous)
		_, _ = hash.Write([]byte(password))
		previous = hash.Sum(nil)
		key = append(key, previous...)
	}
	return key
}

// cfbCipher shares the immutable AES key schedule, never per-connection stream state.
type cfbCipher struct {
	block cipher.Block
}

// StreamConn creates independent send and receive streams for one TCP connection.
func (c *cfbCipher) StreamConn(conn net.Conn) net.Conn {
	return &cfbConn{Conn: conn, block: c.block}
}

// cfbConn implements legacy Shadowsocks: a random 16-byte IV followed by CFB128 data.
// CFB provides encryption only, with no authentication tag or replay protection.
type cfbConn struct {
	net.Conn
	block    cipher.Block
	readMu   sync.Mutex
	writeMu  sync.Mutex
	decoder  cipher.Stream
	encoder  cipher.Stream
	readErr  error
	writeErr error
}

// Read consumes the peer's IV once and decrypts the continuous incoming byte stream.
func (c *cfbConn) Read(data []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	if c.readErr != nil {
		return 0, c.readErr
	}
	if c.decoder == nil {
		iv := make([]byte, aes.BlockSize)
		if _, err := io.ReadFull(c.Conn, iv); err != nil {
			c.readErr = err
			return 0, err
		}
		c.decoder = cipher.NewCFBDecrypter(c.block, iv)
	}
	n, err := c.Conn.Read(data)
	c.decoder.XORKeyStream(data[:n], data[:n])
	c.readErr = err
	return n, err
}

// Write emits a fresh IV on first use and reports plaintext bytes, excluding the IV.
// A failed write makes the send stream unusable because CFB state has already advanced.
func (c *cfbConn) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	var iv []byte
	if c.encoder == nil {
		iv = make([]byte, aes.BlockSize)
		if _, err := rand.Read(iv); err != nil {
			c.writeErr = err
			return 0, err
		}
		c.encoder = cipher.NewCFBEncrypter(c.block, iv)
	}
	packet := make([]byte, len(iv)+len(data))
	copy(packet, iv)
	c.encoder.XORKeyStream(packet[len(iv):], data)
	n, err := c.Conn.Write(packet)
	if err == nil && n != len(packet) {
		err = io.ErrShortWrite
	}
	c.writeErr = err
	n -= len(iv)
	if n < 0 {
		n = 0
	}
	return n, err
}
