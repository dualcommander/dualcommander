package commander

import (
	"strings"
	"testing"
)

func TestApplicationVersion(t *testing.T) {
	previous := buildVersion
	t.Cleanup(func() { buildVersion = previous })
	for _, version := range []string{"0.1.0", "1.2.3-rc.1", "0.1.0+abcdef123456", "0.1.0+abcdef123456.dirty"} {
		buildVersion = version
		if got := applicationVersion(); got != version {
			t.Fatalf("got %q, want %q", got, version)
		}
	}
	buildVersion = ""
	if got := applicationVersion(); !strings.HasPrefix(got, "0.1.0+") {
		t.Fatalf("plain Go build missing development metadata: %s", got)
	}
}
