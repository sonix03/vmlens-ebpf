package privacy

import (
	"reflect"
	"testing"
)

func TestSanitizeArgs(t *testing.T) {
	tests := []struct{ in, want []string }{
		{[]string{"mysql", "-u", "root", "-pSuperSecret"}, []string{"mysql", "-u", "root", "-p[REDACTED]"}},
		{[]string{"app", "TOKEN=abc"}, []string{"app", "TOKEN=[REDACTED]"}},
		{[]string{"curl", "--token", "abc"}, []string{"curl", "--token", "[REDACTED]"}},
		{[]string{"curl", "-H", "Authorization: Bearer abc123", "https://example"}, []string{"curl", "-H", "Authorization: Bearer [REDACTED]", "https://example"}},
	}
	for _, tt := range tests {
		if got := SanitizeArgs(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("got %q want %q", got, tt.want)
		}
	}
}
