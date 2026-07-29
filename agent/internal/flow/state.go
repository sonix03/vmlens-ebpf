package flow

import (
	"github.com/vmlens/vmlens/agent/internal/features/classification"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/connection"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/retrans"
	"github.com/vmlens/vmlens/agent/internal/features/protocols/transport/tcp/rtt"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/bytes"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/direction"
	"github.com/vmlens/vmlens/agent/internal/features/traffic/packets"
)

type State struct {
	Key            Key
	Traffic        TrafficState
	TCP            TCPState
	Classification classification.Model
	FirstSeenNS    uint64
	LastSeenNS     uint64
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
