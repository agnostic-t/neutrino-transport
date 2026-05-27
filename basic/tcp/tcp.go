package tcp

import (
	"net"
	"time"

	"github.com/agnostic-t/neutrino-core/transport"
)

var _ transport.Client = (*Client)(nil)
var _ transport.Server = (*Server)(nil)

// ==========================================

type Client struct {
	serverAddr string
	timeout    time.Duration
}

func NewClient(serverAddr string, timeout time.Duration) *Client {
	return &Client{
		serverAddr: serverAddr,
		timeout:    timeout,
	}
}

func (c *Client) Dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", c.serverAddr, c.timeout)
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	return conn, nil
}

// ==========================================

type Server struct {
	bindAddr string
}

func NewServer(bindAddr string) *Server {
	return &Server{
		bindAddr: bindAddr,
	}
}

func (s *Server) Listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", s.bindAddr)
	if err != nil {
		return nil, err
	}

	return listener, nil
}
