package http

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
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

	w  *bufio.Writer
	mu sync.Mutex
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

// func (c *HttpConn) Write(b []byte) (int, error) {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()
// 	req, err := http.NewRequest("GET", c.keyPath, bytes.NewReader(b))
// 	if err != nil {
// 		return 0, err
// 	}
// 	if c.isServer {
// 		setHeadersServer(req, c.origin)
// 	} else {
// 		setHeadersClient(req, c.origin, c.host, c.reqUA)
// 	}
// 	req.ContentLength = int64(len(b))
// 	if err := req.Write(c.w); err != nil {
// 		return 0, err
// 	}
// 	if err := c.w.Flush(); err != nil {
// 		return 0, err
// 	}
// 	return len(b), nil
// }

func (c *HttpConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var headerStr string
	if c.isServer {
		headerStr = fmt.Sprintf("GET %s HTTP/1.1\r\n"+
			"Cache-Control: no-store\r\n"+
			"Content-Type: application/octet-stream\r\n"+
			"Access-Control-Allow-Origin: %s\r\n"+
			"Content-Length: %d\r\n\r\n", c.keyPath, c.origin, len(b))
	} else {
		headerStr = fmt.Sprintf("GET %s HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: %s\r\n"+
			"Sec-Fetch-Site: cross-site\r\n"+
			"Sec-Fetch-Mode: cors\r\n"+
			"Sec-Fetch-Dest: empty\r\n"+
			"Referer: %s\r\n"+
			"Accept-Language: en-Us,en;q=0.9\r\n"+
			"Content-Length: %d\r\n\r\n", c.keyPath, c.host, c.reqUA, c.origin, len(b))
	}

	if _, err := c.w.WriteString(headerStr); err != nil {
		return 0, err
	}

	if _, err := c.w.Write(b); err != nil {
		return 0, err
	}

	if err := c.w.Flush(); err != nil {
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

func (c *HttpConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
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
		w:        bufio.NewWriter(conn),
	}, nil
}

type HttpListener struct {
	net.Listener
	origin  string
	keyPath string

	errs  chan error
	conns chan net.Conn
}

func sendNginxResponse(conn net.Conn, statusCode int) {
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})

	resp := nginxGenPage(statusCode)
	_, _ = conn.Write([]byte(resp))
}

func (l *HttpListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case err := <-l.errs:
		return nil, err
	}
}

func (l *HttpListener) startAcceptLoop() {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			l.errs <- err
			return
		}
		go l.handleHandshake(conn)
	}
}

func (l *HttpListener) handleHandshake(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)

	req, err := http.ReadRequest(reader)
	if err != nil {
		fmt.Printf("[http] failed to parse request: %w\n", err)
		sendNginxResponse(conn, 400)
		conn.Close()
		return
	}

	if req.URL.Path != l.keyPath {
		fmt.Printf("[http] Request path is not key path: %s != %s\n", req.URL.Path, l.keyPath)
		sendNginxResponse(conn, 404)
		conn.Close()
		return
	}

	conn.SetReadDeadline(time.Time{})

	l.conns <- &HttpConn{
		Conn:     conn,
		r:        reader,
		body:     req.Body,
		isServer: true,
		origin:   l.origin,
		keyPath:  l.keyPath,
		w:        bufio.NewWriter(conn),
	}
}

func (s *Server) Listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", s.bindAddr)
	if err != nil {
		return nil, err
	}

	lstn := &HttpListener{
		Listener: listener,
		origin:   s.origin,
		keyPath:  s.keyPath,
		conns:    make(chan net.Conn, 128),
		errs:     make(chan error, 1),
	}
	go lstn.startAcceptLoop()
	return lstn, nil
}
