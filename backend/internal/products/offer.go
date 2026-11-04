package products

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	offerURLPattern    = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*://)([^/?#]+)([^?#]*)(\?[^#]*)?$`)
	offerHostPattern   = regexp.MustCompile(`^(?:\[[^]]+\](?::[0-9]+)?|[^:\s\[\]]+(?::[0-9]+)?)$`)
	offerZonePattern   = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
	invalidEscape      = regexp.MustCompile(`%([^0-9A-Fa-f]|$)|%[0-9A-Fa-f]([^0-9A-Fa-f]|$)`)
	errInvalidCurrency = errors.New("currency must contain exactly three uppercase ASCII letters")
)

func ValidateCurrencyCode(currency string) error {
	if len(currency) != 3 {
		return errInvalidCurrency
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return errInvalidCurrency
		}
	}
	return nil
}

func NormalizeProductOfferURL(value string) string {
	trimmedURL := strings.Trim(value, " ")
	urlWithoutFragment := strings.SplitN(trimmedURL, "#", 2)[0]
	parts := offerURLPattern.FindStringSubmatch(urlWithoutFragment)
	if parts == nil || strings.ContainsAny(parts[2], " \t\r\n") || invalidEscape.MatchString(trimmedURL) {
		return trimmedURL
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return trimmedURL
	}

	authority := parts[2]
	userInfo := ""
	hostPort := authority
	if separator := strings.LastIndex(authority, "@"); separator >= 0 {
		userInfo = authority[:separator+1]
		hostPort = authority[separator+1:]
	}
	if !offerHostPattern.MatchString(hostPort) || !validOfferHost(hostPort) {
		return trimmedURL
	}

	path := strings.TrimRight(parts[3], "/")
	return strings.ToLower(parts[1]) + userInfo + strings.ToLower(hostPort) + path + parts[4]
}

func validOfferHost(hostPort string) bool {
	if !strings.HasPrefix(hostPort, "[") {
		return true
	}
	closeBracket := strings.IndexByte(hostPort, ']')
	if closeBracket < 0 {
		return false
	}
	host := hostPort[1:closeBracket]
	if zoneStart := strings.Index(host, "%25"); zoneStart >= 0 {
		zone := host[zoneStart+3:]
		if !offerZonePattern.MatchString(zone) {
			return false
		}
		host = host[:zoneStart]
	} else if strings.Contains(host, "%") {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() == nil
}
