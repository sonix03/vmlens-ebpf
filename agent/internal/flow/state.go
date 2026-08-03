package flow

import (
	"time"

	"github.com/vmlens/vmlens/agent/internal/features/classification"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/application/http"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/connection"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/retrans"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/rtt"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/bytes"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/packets"
)

type State struct {
	Key            Key
	SrcPort        int
	Traffic        TrafficState
	TCP            TCPState
	Application    ApplicationState
	Classification classification.Model
	FirstSeen      time.Time
	LastSeen       time.Time
}

type TrafficState struct {
	Bytes     bytes.Model
	Packets   packets.Model
	Direction direction.Model
}

type TCPState struct {
	Connection connection.Model
	RTT        rtt.Model
	Retrans    retrans.Model
}

type ApplicationState struct {
	AvgDelayMs float64
	Samples    uint64
	HTTPStatus http.StatusCounters
}
