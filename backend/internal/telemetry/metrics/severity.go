package metrics

func ConnectionSeverity(success bool, rttMs float64, errorCount int64, slowRTTThresholdMs float64) string {
	if !success || errorCount > 0 {
		return SeverityError
	}
	if slowRTTThresholdMs > 0 && rttMs >= slowRTTThresholdMs {
		return SeverityWarning
	}
	return SeverityNormal
}
