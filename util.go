package main

import (
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
)

// formatBaseURL takes a hostname (baseHost) and a base path
// and joins them.  Both are parsed as URLs (using net/url) and
// then joined to ensure a properly formed URL.
// net/url does not support parsing hostnames without a scheme
// (e.g. example.com is invalid; http://example.com is valid).
// serverURLHost ensures a scheme is added.
func formatBaseURL(baseHost string, basePath string) string {
	urlHost, err := url.Parse(baseHost)
	if err != nil {
		log.Fatal(err)
	}
	urlPath, err := url.Parse(basePath)
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimRight(urlHost.ResolveReference(urlPath).String(), "/")
}
