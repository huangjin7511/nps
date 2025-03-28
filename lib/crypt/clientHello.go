package crypt

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const defaultMaxSize = 8192

type SniffConn struct {
	net.Conn
	mu           sync.Mutex
	buf          []byte
	maxSize      int
	limitReached bool
}

func NewSniffConn(conn net.Conn, maxSize int) *SniffConn {
	return &SniffConn{
		Conn:         conn,
		buf:          make([]byte, 0),
		maxSize:      maxSize,
		limitReached: false,
	}
}

func (s *SniffConn) Read(p []byte) (int, error) {
	s.mu.Lock()
	if s.limitReached {
		s.mu.Unlock()
		return 0, io.EOF
	}
	s.mu.Unlock()

	n, err := s.Conn.Read(p)
	if n > 0 {
		s.mu.Lock()
		if len(s.buf)+n >= s.maxSize {
			s.buf = append(s.buf, p[:n]...)
			s.limitReached = true
			s.mu.Unlock()
			return n, io.EOF
		}
		s.buf = append(s.buf, p[:n]...)
		s.mu.Unlock()
	}
	return n, err
}

func (s *SniffConn) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf
}

type ReadOnlyConn struct {
	r          *SniffConn
	remoteAddr net.Addr
}

func (c *ReadOnlyConn) Read(p []byte) (int, error)         { return c.r.Read(p) }
func (c *ReadOnlyConn) Write(_ []byte) (int, error)        { return 0, errors.New("readOnlyConn: write not allowed") }
func (c *ReadOnlyConn) Close() error                       { return nil }
func (c *ReadOnlyConn) LocalAddr() net.Addr                { return nil }
func (c *ReadOnlyConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *ReadOnlyConn) SetDeadline(_ time.Time) error      { return nil }
func (c *ReadOnlyConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *ReadOnlyConn) SetWriteDeadline(_ time.Time) error { return nil }

func ReadClientHello(clientConn net.Conn) (helloInfo *tls.ClientHelloInfo, rawData []byte, err error) {
	sconn := NewSniffConn(clientConn, defaultMaxSize)

	roc := &ReadOnlyConn{
		r:          sconn,
		remoteAddr: clientConn.RemoteAddr(),
	}

	var helloInfoPtr *tls.ClientHelloInfo

	fakeTLS := tls.Server(roc, &tls.Config{
		GetConfigForClient: func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
			tmp := *hi
			helloInfoPtr = &tmp
			return nil, nil
		},
	})
	err = fakeTLS.Handshake()
	if helloInfoPtr == nil {
		if err == nil {
			err = errors.New("no clientHello, but handshake returned nil error")
		}
		return nil, sconn.Bytes(), err
	}

	return helloInfoPtr, sconn.Bytes(), nil
}
