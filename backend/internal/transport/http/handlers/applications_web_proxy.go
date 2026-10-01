package httphandlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func newWebProxy(upstream *url.URL, prefix string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, prefix)
			pr.Out.URL.RawPath = ""
			if pr.Out.URL.Path == "" {
				pr.Out.URL.Path = "/"
			}
			pr.Out.Header.Del("Cookie")
			pr.SetXForwarded()
		},
		Transport: &http.Transport{DisableKeepAlives: true, Proxy: nil},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			// An application shares Remote's origin. Never let its service worker
			// claim the main UI or another project's application routes.
			if response.Header.Get("Service-Worker-Allowed") != "" {
				response.Header.Set("Service-Worker-Allowed", prefix+"/")
			}
			if location := response.Header.Get("Location"); strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") {
				response.Header.Set("Location", prefix+location)
			}
			return nil
		},
	}
}
