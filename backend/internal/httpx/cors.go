package httpx

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

func CORS(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")
			if origin == "" {
				w.Header().Set("Access-Control-Allow-Origin", frontendURL)
			} else {
				w.Header().Add("Vary", "Origin")
				if origin == frontendURL || isLocalFrontendOrigin(frontendURL, origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func isLocalFrontendOrigin(frontendURL, origin string) bool {
	frontend, err := url.Parse(frontendURL)
	if err != nil || frontend.Scheme != "http" || !isLoopbackHost(frontend.Hostname()) {
		return false
	}
	requestOrigin, err := url.Parse(origin)
	if err != nil || requestOrigin.Scheme != "http" || requestOrigin.User != nil ||
		requestOrigin.Path != "" || requestOrigin.RawQuery != "" || requestOrigin.Fragment != "" ||
		requestOrigin.Port() != frontend.Port() {
		return false
	}

	host := strings.ToLower(requestOrigin.Hostname())
	if isLoopbackHost(host) {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() != nil && isPrivateIPv4(ip)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isPrivateIPv4(ip net.IP) bool {
	return ip.To4() != nil && ip.IsPrivate()
}
