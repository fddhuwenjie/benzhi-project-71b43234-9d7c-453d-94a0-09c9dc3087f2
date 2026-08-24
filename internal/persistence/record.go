package persistence

import (
	"encoding/json"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

const snapshotFormat = "benzhi-tree-migration-v1"

type CommandResult struct {
	RequestID     string          `json:"request_id"`
	Operation     string          `json:"operation"`
	ApplicationID string          `json:"application_id"`
	Revision      int             `json:"revision"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	RecordedAt    time.Time       `json:"recorded_at"`
}

type snapshotData struct {
	Format      string                       `json:"format"`
	Application *domain.MigrationApplication `json:"application"`
	Commands    map[string]CommandResult     `json:"commands"`
}

type snapshotEnvelope struct {
	Digest string          `json:"digest"`
	Data   json.RawMessage `json:"data"`
}

type EventRecord struct {
	Sequence       int64         `json:"sequence"`
	ApplicationID  string        `json:"application_id"`
	Revision       int           `json:"revision"`
	Operation      string        `json:"operation"`
	RequestID      string        `json:"request_id,omitempty"`
	Status         domain.Status `json:"status"`
	At             time.Time     `json:"at"`
	SnapshotDigest string        `json:"snapshot_digest"`
}

func commandKey(operation, requestID string) string { return operation + "\x1f" + requestID }
