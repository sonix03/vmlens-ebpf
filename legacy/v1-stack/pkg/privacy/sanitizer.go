package privacy

import (
	"regexp"
	"strings"
)

const Redacted = "[REDACTED]"

var sensitiveKey = regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|access[_-]?key|private[_-]?key|bearer|authorization|cookie|session|credential)`)
var assignment = regexp.MustCompile(`^([^=]+)=(.*)$`)

// SanitizeArgs preserves executable observability while removing values likely
// to contain credentials. It deliberately favors false positives over leakage.
func SanitizeArgs(args []string) []string {
	out := append([]string(nil), args...)
	redactNext := false
	for i, arg := range out {
		if redactNext {
			out[i] = Redacted
			redactNext = false
			continue
		}
		lower := strings.ToLower(arg)
		if m := assignment.FindStringSubmatch(arg); len(m) == 3 && sensitiveKey.MatchString(m[1]) {
			out[i] = m[1] + "=" + Redacted
			continue
		}
		if lower == "-p" || sensitiveKey.MatchString(strings.TrimLeft(lower, "-")) && strings.HasPrefix(lower, "-") {
			if strings.Contains(arg, "=") {
				out[i] = arg[:strings.Index(arg, "=")+1] + Redacted
			} else if lower != "-p" && strings.HasPrefix(lower, "-p") && len(arg) > 2 {
				out[i] = arg[:2] + Redacted
			} else {
				redactNext = true
			}
			continue
		}
		if strings.HasPrefix(lower, "-p") && len(arg) > 2 {
			out[i] = arg[:2] + Redacted
			continue
		}
		if sensitiveKey.MatchString(arg) {
			if idx := strings.Index(strings.ToLower(arg), "bearer "); idx >= 0 {
				out[i] = arg[:idx+len("bearer ")] + Redacted
			} else if idx := strings.Index(arg, ":"); idx >= 0 {
				out[i] = arg[:idx+1] + " " + Redacted
			} else if i > 0 {
				out[i] = Redacted
			}
		}
	}
	return out
}

func SanitizeCommand(args []string) string { 
	return strings.Join(SanitizeArgs(args), " ") 
}
