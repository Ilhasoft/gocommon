package urns

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

const (
	// TelScheme is the scheme used for telephone numbers
	TelScheme string = "tel"

	// FacebookScheme is the scheme used for Facebook identifiers
	FacebookScheme string = "facebook"

	// TelegramScheme is the scheme used for Telegram identifiers
	TelegramScheme string = "telegram"

	// TwitterScheme is the scheme used for Twitter handles
	TwitterScheme string = "twitter"

	// TwitterIDScheme is the scheme used for Twitter user ids
	TwitterIDScheme string = "twitterid"

	// ViberScheme is the scheme used for Viber identifiers
	ViberScheme string = "viber"

	// LineScheme is the scheme used for LINE identifiers
	LineScheme string = "line"

	// JiochatScheme is the scheme used for Jiochat identifiers
	JiochatScheme string = "jiochat"

	// EmailScheme is the scheme used for email addresses
	EmailScheme string = "mailto"

	// ExternalScheme is the scheme used for externally defined identifiers
	ExternalScheme string = "ext"
)

var validSchemes = map[string]bool{
	TelScheme:       true,
	FacebookScheme:  true,
	TelegramScheme:  true,
	TwitterScheme:   true,
	TwitterIDScheme: true,
	ViberScheme:     true,
	LineScheme:      true,
	JiochatScheme:   true,
	EmailScheme:     true,
	ExternalScheme:  true,
}

var telRegex = regexp.MustCompile(`[^0-9a-z]`)

// URN represents a Universal Resource Name, we use this for contact identifiers like phone numbers etc..
type URN string

// Path returns the path portion for the URN
func (u URN) Path() string {
	parts := strings.SplitN(string(u), ":", 2)
	if len(parts) == 2 {
		pathParts := strings.SplitN(parts[1], "#", 2)
		if len(pathParts) == 2 {
			return pathParts[0]
		}
		return parts[1]
	}
	return string(u)
}

// Scheme returns the scheme portion for the URN
func (u URN) Scheme() string {
	parts := strings.SplitN(string(u), ":", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// Display returns the display portion for the URN (if any)
func (u URN) Display() string {
	parts := strings.SplitN(string(u), ":", 2)
	if len(parts) == 2 {
		pathParts := strings.SplitN(parts[1], "#", 2)
		if len(pathParts) == 2 {
			return pathParts[1]
		}
	}
	return ""
}

// Identity returns the URN with any display attributes stripped
func (u URN) Identity() string {
	parts := strings.SplitN(string(u), "#", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return string(u)
}

// String returns a string representation of our URN
func (u URN) String() string {
	return string(u)
}

// NilURN is our constant for nil URNs
var NilURN = URN("")

// NewTelURNForCountry returns a URN for the passed in telephone number and country code ("US")
func NewTelURNForCountry(number string, country string) URN {
	// add on a plus if it looks like it could be a fully qualified number
	number = telRegex.ReplaceAllString(strings.ToLower(strings.TrimSpace(number)), "")
	parseNumber := number
	if len(number) >= 11 && !(strings.HasPrefix(number, "+") || strings.HasPrefix(number, "0")) {
		parseNumber = fmt.Sprintf("+%s", number)
	}

	normalized, err := phonenumbers.Parse(parseNumber, country)

	// couldn't parse it, use the original number
	if err != nil {
		return newURN(TelScheme, number, "")
	}

	// if it looks valid, return it
	if phonenumbers.IsValidNumber(normalized) {
		return newURN(TelScheme, phonenumbers.Format(normalized, phonenumbers.E164), "")
	}

	// this doesn't look like anything we recognize, use the original number
	return newURN(TelScheme, number, "")
}

// NewTelegramURN returns a URN for the passed in telegram identifier
func NewTelegramURN(identifier int64, display string) URN {
	return newURN(TelegramScheme, strconv.FormatInt(identifier, 10), display)
}

// NewURNFromParts returns a new URN for the given scheme, path and display
func NewURNFromParts(scheme string, path string, display string) (URN, error) {
	scheme = strings.ToLower(scheme)
	if !validSchemes[scheme] {
		return NilURN, fmt.Errorf("invalid scheme '%s'", scheme)
	}

	return newURN(scheme, path, display), nil
}

// private utility method to create a URN from a scheme and path
func newURN(scheme string, path string, display string) URN {
	urn := fmt.Sprintf("%s:%s", scheme, path)
	if display != "" {
		urn = fmt.Sprintf("%s#%s", urn, strings.ToLower(display))
	}
	return URN(urn)
}
