package http

import (
	"fmt"
	"net/http"
	"time"
)

func nginxGenPage(statusCode int) string {
	var body, statusText, dateStr string

	dateStr = time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")

	switch statusCode {
	case 400:
		statusText = "400 Bad Request"
		body = "<html>\r\n<head><title>400 Bad Request</title></head>\r\n<body>\r\n<center><h1>400 Bad Request</h1></center>\r\n<hr><center>nginx</center>\r\n</body>\r\n</html>\r\n"
	case 404:
		statusText = "404 Not Found"
		body = "<html>\r\n<head><title>404 Not Found</title></head>\r\n<body>\r\n<center><h1>404 Not Found</h1></center>\r\n<hr><center>nginx</center>\r\n</body>\r\n</html>\r\n"
	default: // 502
		statusText = "502 Bad Gateway"
		body = "<html>\r\n<head><title>502 Bad Gateway</title></head>\r\n<body>\r\n<center><h1>502 Bad Gateway</h1></center>\r\n<hr><center>nginx</center>\r\n</body>\r\n</html>\r\n"
	}

	return fmt.Sprintf(
		"HTTP/1.1 %s\r\n"+
			"Server: nginx\r\n"+
			"Date: %s\r\n"+
			"Content-Type: text/html\r\n"+
			"Content-Length: %d\r\n"+
			"Connection: close\r\n\r\n"+
			"%s",
		statusText, dateStr, len(body), body,
	)
}

func setHeadersClient(
	req *http.Request,
	referer string,
	host string,
	userAgent string,
) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept-Language", "en-Us,en;q=0.9")
	req.Header.Set("Host", host)
}

func setHeadersServer(
	req *http.Request,
	originAllowed string,
) {
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Access-Control-Allow-Origin", originAllowed)
}
