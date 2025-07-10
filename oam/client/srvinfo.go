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

func (c *Client) parseServer(args []string) *ServerInfo {
	if len(args) == 0 {
		return nil
	}
	srv := &ServerInfo{
		url: "http://" + args[0],
	}
	if len(args) >= 2 {
		srv.headers = parseHeaders(args[1])
	}
	var certName string
	if len(args) >= 3 {
		certName = args[2]
	}

	if len(certName) > 0 {
		srv.cli = httpw.NewClient(c.cert, c.caPool, certName)
	} else {
		srv.cli = httpw.NewClient(nil, nil, "")
	}

	return srv
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
