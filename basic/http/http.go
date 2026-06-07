package http

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/agnostic-t/neutrino-core/transport"
)

var _ transport.Client = (*Client)(nil)
var _ transport.Server = (*Server)(nil)

type Client struct {
	serverAddr string
	timeout    time.Duration

	userAgent string
	referer   string
	keyPath   string
}

type Server struct {
	bindAddr string
	origin   string
	keyPath  string
}

func NewServer(bindAddr, keyPath string, origin string) *Server {
	return &Server{
		bindAddr: bindAddr,
		origin:   origin,
		keyPath:  keyPath,
	}
}

func NewClient(serverAddr, referer, userAgent, keyPath string, timeout time.Duration) *Client {
	return &Client{
		serverAddr: serverAddr,
		timeout:    timeout,
		userAgent:  userAgent,
		referer:    referer,
		keyPath:    keyPath,
	}
}

type HttpConn struct {
	net.Conn
	r    *bufio.Reader
	body io.ReadCloser

	isServer bool
	reqUA    string

	origin  string
	host    string
	keyPath string
}

func (c *HttpConn) Read(b []byte) (int, error) {
	for {
		if c.body != nil {
			n, err := c.body.Read(b)
			if err == io.EOF {
				c.body.Close()
				c.body = nil
				if n > 0 {
					return n, nil
				}
				continue
			}
			return n, err
		}

		req, err := http.ReadRequest(c.r)
		if err != nil {
			return 0, err
		}
		c.body = req.Body
	}
}

func (c *HttpConn) Write(b []byte) (int, error) {
	req, err := http.NewRequest("GET", c.keyPath, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	if c.isServer {
		setHeadersServer(req, c.origin)
	} else {
		setHeadersClient(req, c.origin, c.host, c.reqUA)
	}
	req.ContentLength = int64(len(b))

	w := bufio.NewWriter(c.Conn)
	if err := req.Write(w); err != nil {
		return 0, err
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *HttpConn) Close() error {
	if c.body != nil {
		c.body.Close()
	}
	return c.Conn.Close()
}

func (c *Client) Dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", c.serverAddr, c.timeout)
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	return &HttpConn{
		Conn:     conn,
		r:        bufio.NewReader(conn),
		isServer: false,
		keyPath:  c.keyPath,
		reqUA:    c.userAgent,
		host:     c.serverAddr,
		origin:   c.referer,
	}, nil
}

type HttpListener struct {
	net.Listener
	origin  string
	keyPath string
}

func sendNginuxResponse(conn net.Conn, statusCode int) {
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})

	resp := nginxGenPage(statusCode)
	_, _ = conn.Write([]byte(resp))
}

func (l *HttpListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		reader := bufio.NewReader(conn)

		req, err := http.ReadRequest(reader)
		if err != nil {
			fmt.Printf("[http] failed to parse request: %w\n", err)
			sendNginuxResponse(conn, 400)
			conn.Close()
			continue
		}

		if req.URL.Path != l.keyPath {
			fmt.Printf("[http] Request path is not key path: %s != %s\n", req.URL.Path, l.keyPath)
			sendNginuxResponse(conn, 404)
			conn.Close()
			continue
		}

		conn.SetReadDeadline(time.Time{})

		return &HttpConn{
			Conn:     conn,
			r:        reader,
			body:     req.Body,
			isServer: true,
			origin:   l.origin,
			keyPath:  l.keyPath,
		}, nil
	}
}

func (s *Server) Listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", s.bindAddr)
	if err != nil {
		return nil, err
	}

	return &HttpListener{
		Listener: listener,
		origin:   s.origin,
		keyPath:  s.keyPath,
	}, nil
}
