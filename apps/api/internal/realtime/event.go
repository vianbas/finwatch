// Package realtime relays committed outbox events to connected WebSocket
// clients through an in-process hub (ADR 0003). Delivery is at-least-once and
// only to clients connected at publish time; there is no replay.
package realtime

import (
	"encoding/json"
	"strconv"
	"time"
)

// envelopeVersion is the schema version of every event's data payload.
const envelopeVersion = 1

// Event is one outbox row ready to be relayed.
type Event struct {
	// ID is the outbox row id; it is stable across redeliveries.
	ID         int64
	Type       string
	Payload    json.RawMessage
	OccurredAt time.Time
}

// envelope is the wire form of an Event. It matches the Envelope schema in
// contracts/asyncapi.yaml.
type envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt string          `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}

// MarshalEnvelope renders e as the contract's common event envelope.
func MarshalEnvelope(e Event) ([]byte, error) {
	return json.Marshal(envelope{
		ID:         strconv.FormatInt(e.ID, 10),
		Type:       e.Type,
		Version:    envelopeVersion,
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339Nano),
		Data:       e.Payload,
	})
}
