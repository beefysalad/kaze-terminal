package discord

import (
	"regexp"
	"testing"
)

// A Discord Application ID is a numeric snowflake of 17 to 20 digits. An empty
// ID means the build never talks to Discord.
func TestClientIDIsAnApplicationID(t *testing.T) {
	if !regexp.MustCompile(`^[0-9]{17,20}$`).MatchString(ClientID) {
		t.Fatalf("ClientID %q is not a Discord Application ID", ClientID)
	}
}
