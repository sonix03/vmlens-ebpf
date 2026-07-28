package metrics

import "github.com/vmlens/vmlens/backend/internal/model"

func TotalBytes(flow model.FlowEvent) int64 {
	return flow.BytesSent + flow.BytesReceived
}
