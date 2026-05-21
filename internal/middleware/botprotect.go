package middleware

import (
	"net/http"
	"strings"
)

// blockedUAPatterns holds lower-cased substrings found in known crawler/
// scraper/security-scanner User-Agent strings.
var blockedUAPatterns = []string{
	"scrapy",
	"wget",
	"python-requests",
	"go-http-client",
	"libwww-perl",
	"masscan",
	"zgrab",
	"sqlmap",
	"nmap",
	"nikto",
	"ahrefsbot",
	"semrushbot",
	"dotbot",
	"mj12bot",
	"baiduspider",
	"yandexbot",
	"petalbot",
	"bytespider",
}

// BotProtection is middleware that rejects requests with no User-Agent or with
// a User-Agent matching a known bot/scraper pattern.
func BotProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := strings.ToLower(r.Header.Get("User-Agent"))

		if ua == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		for _, pattern := range blockedUAPatterns {
			if strings.Contains(ua, pattern) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
