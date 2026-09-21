package observability

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	hostSessionStateKey    = "host.session.v1"
	recoveryEventType      = "unexpected_shutdown_recovery"
	recoveryIncidentStatus = "recovered"
)

var (
	ErrInvalidHostObservation = errors.New("invalid host observation")
	ErrInvalidPreviousSession = errors.New("invalid previous host session")
	ErrInvalidRecoveryClock   = errors.New("recovery time precedes last known alive time")
)

type HostSession struct {
	BootID        string    `json:"bootId"`
	BootedAt      time.Time `json:"bootedAt"`
	LastSeenAt    time.Time `json:"lastSeenAt"`
	CleanShutdown bool      `json:"cleanShutdown"`
}

type HostSessionUpdate struct {
	StateValue string
	Event      *Event
	Incident   *RecoveryIncident
	Critical   bool
}

type HostSessionTracker struct {
	session  *HostSession
	observed bool
}

func (t *HostSessionTracker) SetBaseline(value string) error {
	session, err := decodeHostSession(value)
	if err != nil {
		return err
	}
	t.session = &session
	t.observed = false
	return nil
}

func (t *HostSessionTracker) Observe(
	bootID string,
	observedAt time.Time,
	uptimeSeconds float64,
) (HostSessionUpdate, error) {
	bootID = strings.TrimSpace(bootID)
	bootedAt, ok := recoveredAtFromUptime(observedAt, uptimeSeconds)
	if bootID == "" || !ok {
		return HostSessionUpdate{}, ErrInvalidHostObservation
	}

	current := HostSession{
		BootID:        bootID,
		BootedAt:      bootedAt,
		LastSeenAt:    observedAt.UTC(),
		CleanShutdown: false,
	}
	previous := t.session
	if previous != nil && previous.BootID == bootID {
		current.BootedAt = previous.BootedAt
	}

	update := HostSessionUpdate{
		Critical: previous == nil || previous.BootID != bootID || previous.CleanShutdown,
	}
	var observationErr error
	if previous != nil && previous.BootID != bootID && !previous.CleanShutdown {
		switch {
		case previous.BootID == "" || previous.LastSeenAt.IsZero():
			observationErr = ErrInvalidPreviousSession
		case bootedAt.Before(previous.LastSeenAt):
			observationErr = ErrInvalidRecoveryClock
		default:
			downtime := int64(bootedAt.Sub(previous.LastSeenAt) / time.Second)
			sourceID := "recovery:" + bootID
			event := newEvent(
				"reactorlab",
				sourceID,
				recoveryEventType,
				"host",
				"dell",
				"Dell host",
				bootedAt,
				"Dell host recovered after an unexpected shutdown",
				map[string]any{
					"lastKnownAliveAt": previous.LastSeenAt.UTC(),
					"recoveredAt":      bootedAt,
					"downtimeSeconds":  downtime,
					"status":           recoveryIncidentStatus,
					"previousBootId":   previous.BootID,
					"recoveryBootId":   bootID,
				},
			)
			incident := RecoveryIncident{
				EventID:          event.ID,
				LastKnownAliveAt: previous.LastSeenAt.UTC(),
				RecoveredAt:      bootedAt,
				DowntimeSeconds:  downtime,
				Status:           recoveryIncidentStatus,
				PreviousBootID:   previous.BootID,
				RecoveryBootID:   bootID,
			}
			update.Event = &event
			update.Incident = &incident
		}
	}

	t.session = &current
	t.observed = true
	encoded, err := json.Marshal(current)
	if err != nil {
		return HostSessionUpdate{}, fmt.Errorf("encode host session: %w", err)
	}
	update.StateValue = string(encoded)
	return update, observationErr
}

func (t *HostSessionTracker) MarkClean() (string, bool, error) {
	if !t.observed || t.session == nil || strings.TrimSpace(t.session.BootID) == "" {
		return "", false, nil
	}
	t.session.CleanShutdown = true
	encoded, err := json.Marshal(t.session)
	if err != nil {
		return "", false, fmt.Errorf("encode clean host session: %w", err)
	}
	return string(encoded), true, nil
}

func decodeHostSession(value string) (HostSession, error) {
	var session HostSession
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return HostSession{}, fmt.Errorf("decode host session: %w", err)
	}
	session.BootID = strings.TrimSpace(session.BootID)
	session.BootedAt = session.BootedAt.UTC()
	session.LastSeenAt = session.LastSeenAt.UTC()
	if session.BootID == "" || session.BootedAt.IsZero() || session.LastSeenAt.IsZero() ||
		session.LastSeenAt.Before(session.BootedAt) {
		return HostSession{}, ErrInvalidPreviousSession
	}
	return session, nil
}

func recoveredAtFromUptime(observedAt time.Time, uptimeSeconds float64) (time.Time, bool) {
	if observedAt.IsZero() || uptimeSeconds < 0 || math.IsNaN(uptimeSeconds) || math.IsInf(uptimeSeconds, 0) {
		return time.Time{}, false
	}
	nanoseconds := uptimeSeconds * float64(time.Second)
	if nanoseconds > float64(math.MaxInt64) {
		return time.Time{}, false
	}
	return observedAt.UTC().Add(-time.Duration(nanoseconds)), true
}
