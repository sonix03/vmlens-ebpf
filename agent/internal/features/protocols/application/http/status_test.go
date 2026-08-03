package http

import "testing"

func TestApplyStatusCountsByClass(t *testing.T) {
	var counters StatusCounters
	for _, status := range []int{200, 204, 301, 404, 500, 503} {
		ApplyStatus(&counters, status)
	}
	if counters.Success != 2 {
		t.Fatalf("2xx = %d, want 2", counters.Success)
	}
	if counters.Redirect != 1 {
		t.Fatalf("3xx = %d, want 1", counters.Redirect)
	}
	if counters.ClientError != 1 {
		t.Fatalf("4xx = %d, want 1", counters.ClientError)
	}
	if counters.ServerError != 2 {
		t.Fatalf("5xx = %d, want 2", counters.ServerError)
	}
	if counters.Last != 503 {
		t.Fatalf("last = %d, want 503", counters.Last)
	}
	if counters.Total() != 6 {
		t.Fatalf("total = %d, want 6", counters.Total())
	}
}

func TestApplyStatusIgnoresPacketsWithoutStatusLine(t *testing.T) {
	var counters StatusCounters
	// 0 is what the kernel reports for a packet carrying no status line.
	for _, status := range []int{0, 99, 600, -1} {
		ApplyStatus(&counters, status)
	}
	if counters.Total() != 0 || counters.Last != 0 {
		t.Fatalf("counters moved on non-status input: %+v", counters)
	}
}

func TestMergeStatusKeepsLatestCode(t *testing.T) {
	current := StatusCounters{Success: 2, Last: 200}
	MergeStatus(&current, StatusCounters{ServerError: 1, Last: 502})
	if current.Success != 2 || current.ServerError != 1 {
		t.Fatalf("merged counts = %+v", current)
	}
	if current.Last != 502 {
		t.Fatalf("last = %d, want 502", current.Last)
	}
}

func TestMergeStatusKeepsExistingCodeWhenNextIsEmpty(t *testing.T) {
	current := StatusCounters{Success: 1, Last: 200}
	MergeStatus(&current, StatusCounters{})
	if current.Last != 200 {
		t.Fatalf("last = %d, want 200", current.Last)
	}
}
