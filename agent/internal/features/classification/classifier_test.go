package classification

import "testing"

func TestApplicationFromPort(t *testing.T) {
	if got := ApplicationFromPort(ProtocolTCP, 5432); got != ApplicationPostgreSQL {
		t.Fatalf("ApplicationFromPort(tcp,5432)=%q", got)
	}
}
