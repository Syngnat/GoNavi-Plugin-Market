package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The plugin ships no third-party modules, so the Chrome DevTools transport is a
// minimal RFC 6455 client. It speaks only what CDP needs: one text frame out, text
// frames in, and control frames handled without tearing the connection down.
const (
	wsAcceptGUID  = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	wsMaxFrame    = 1 << 20
	wsDialTimeout = 5 * time.Second

	wsOpcodeText  = 0x1
	wsOpcodeClose = 0x8
	wsOpcodePing  = 0x9
	wsOpcodePong  = 0xa
)

var (
	errWebSocketHandshake = errors.New("websocket handshake rejected")
	errWebSocketProtocol  = errors.New("websocket protocol violation")
	errWebSocketTooLarge  = errors.New("websocket frame exceeds limit")
)

type wsConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

func (c *wsConn) Close() error { return c.conn.Close() }

// dialWebSocket opens the loopback DevTools endpoint. Only plain ws is accepted:
// CDP publishes ws:// on 127.0.0.1 and never offers a TLS variant.
func dialWebSocket(raw string) (*wsConn, error) {
	target, err := url.Parse(raw)
	if err != nil || target.Host == "" || target.Scheme != "ws" || target.User != nil || len(target.RawQuery) > 512 || len(raw) > 2048 {
		return nil, errWebSocketHandshake
	}
	key, err := wsKey()
	if err != nil {
		return nil, errWebSocketHandshake
	}
	conn, err := net.DialTimeout("tcp", target.Host, wsDialTimeout)
	if err != nil {
		return nil, errWebSocketHandshake
	}
	request := "GET " + target.RequestURI() + " HTTP/1.1\r\n" +
		"Host: " + target.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		_ = conn.Close()
		return nil, errWebSocketHandshake
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		_ = conn.Close()
		return nil, errWebSocketHandshake
	}
	// The 101 response carries no body, so the reader keeps every byte Chrome
	// sends afterwards; the connection is never handed to the response body.
	if response.StatusCode != http.StatusSwitchingProtocols ||
		!strings.EqualFold(response.Header.Get("Upgrade"), "websocket") ||
		response.Header.Get("Sec-WebSocket-Accept") != wsAcceptValue(key) {
		_ = conn.Close()
		return nil, errWebSocketHandshake
	}
	return &wsConn{conn: conn, reader: reader}, nil
}

func wsKey() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(nonce), nil
}

func wsAcceptValue(key string) string {
	digest := sha1.Sum([]byte(key + wsAcceptGUID))
	return base64.StdEncoding.EncodeToString(digest[:])
}

// writeFrame sends one final, masked frame; a client must mask every frame.
func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	if len(payload) > wsMaxFrame {
		return errWebSocketTooLarge
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return errWebSocketProtocol
	}
	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|opcode)
	switch {
	case len(payload) < 126:
		frame = append(frame, byte(len(payload))|0x80)
	case len(payload) <= 0xffff:
		frame = append(frame, 126|0x80)
		frame = binary.BigEndian.AppendUint16(frame, uint16(len(payload)))
	default:
		frame = append(frame, 127|0x80)
		frame = binary.BigEndian.AppendUint64(frame, uint64(len(payload)))
	}
	frame = append(frame, mask...)
	start := len(frame)
	frame = append(frame, payload...)
	for index := range frame[start:] {
		frame[start+index] ^= mask[index%4]
	}
	_, err := c.conn.Write(frame)
	return err
}

// readFrame returns one complete frame. Fragmentation and reserved bits are
// rejected: CDP never uses them, and reassembly would widen the parser for nothing.
func (c *wsConn) readFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return 0, nil, err
	}
	if header[0]&0x70 != 0 || header[0]&0x80 == 0 {
		return 0, nil, errWebSocketProtocol
	}
	// A server frame must not be masked; accepting one would let a local process
	// drive the client's parser with attacker-chosen key bytes.
	if header[1]&0x80 != 0 {
		return 0, nil, errWebSocketProtocol
	}
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, extended); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, extended); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(extended)
	}
	if length > wsMaxFrame {
		return 0, nil, errWebSocketTooLarge
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}
	return header[0] & 0x0f, payload, nil
}

// readText returns the next text payload, answering pings and treating a close
// frame as end of stream.
func (c *wsConn) readText() ([]byte, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch opcode {
		case wsOpcodeText:
			return payload, nil
		case wsOpcodeClose:
			return nil, io.EOF
		case wsOpcodePing:
			if err := c.writeFrame(wsOpcodePong, payload); err != nil {
				return nil, err
			}
		case wsOpcodePong:
		default:
			return nil, errWebSocketProtocol
		}
	}
}
