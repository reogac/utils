package client

import (
	"github.com/reogac/utils/httpw"
	"strings"
)

type ServerInfo struct {
	url     string
	headers map[string]string
	cli     *httpw.Client
}

func (c *Client) initServerInfo(url, headers, certName string) {
	c.server = &ServerInfo{
		url:     "http://" + url,
		headers: parseHeaders(headers),
	}
	if len(certName) > 0 {
		c.server.cli = httpw.NewClient(c.cert, c.caPool, certName)
	} else {
		c.server.cli = httpw.NewClient(nil, nil, "")
	}
}

func parseHeaders(str string) (headers map[string]string) {
	var hList [][2]string
	parts := strings.Split(str, ",")
	for _, item := range parts {
		if k, v := parseHeader(item); len(k) > 0 && len(v) > 0 {
			hList = append(hList, [2]string{k, v})
		}
	}
	if len(hList) > 0 {
		headers = make(map[string]string)
		for _, item := range hList {
			headers[item[0]] = item[1]
		}
	}
	return
}

func parseHeader(str string) (k, v string) {
	parts := strings.Split(str, ":")
	if len(parts) >= 2 {
		k = strings.TrimSpace(parts[0])
		v = strings.TrimSpace(parts[1])
	}
	return
}
