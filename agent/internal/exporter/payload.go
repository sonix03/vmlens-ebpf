package exporter

type Payload interface{}

func FlowPayload(event FlowEvent) FlowEvent {
	return event
}
