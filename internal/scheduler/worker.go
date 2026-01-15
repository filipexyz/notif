package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/jackc/pgx/v5/pgtype"
)

// Worker polls for pending scheduled events and publishes them.
type Worker struct {
	queries   *db.Queries
	publisher *nats.Publisher
	interval  time.Duration
}

// NewWorker creates a new scheduler worker.
func NewWorker(queries *db.Queries, publisher *nats.Publisher, interval time.Duration) *Worker {
	return &Worker{
		queries:   queries,
		publisher: publisher,
		interval:  interval,
	}
}

// Start runs the scheduler worker until the context is cancelled.
func (w *Worker) Start(ctx context.Context) {
	slog.Info("scheduler worker started", "interval", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start
	w.processPending(ctx)

	for {
		select {
		case <-ticker.C:
			w.processPending(ctx)
		case <-ctx.Done():
			slog.Info("scheduler worker stopped")
			return
		}
	}
}

func (w *Worker) processPending(ctx context.Context) {
	events, err := w.queries.GetPendingScheduledEvents(ctx, 100)
	if err != nil {
		slog.Error("failed to get pending scheduled events", "error", err)
		return
	}

	if len(events) == 0 {
		return
	}

	slog.Debug("processing pending scheduled events", "count", len(events))

	for _, sch := range events {
		w.executeScheduled(ctx, sch)
	}
}

func (w *Worker) executeScheduled(ctx context.Context, sch db.ScheduledEvent) {
	// Create event from scheduled event
	event := domain.NewEvent(sch.Topic, json.RawMessage(sch.Data))
	event.OrgID = sch.OrgID

	// Publish to NATS
	if err := w.publisher.Publish(ctx, event); err != nil {
		slog.Error("failed to publish scheduled event",
			"scheduled_id", sch.ID,
			"topic", sch.Topic,
			"error", err,
		)
		w.queries.UpdateScheduledEventStatus(ctx, db.UpdateScheduledEventStatusParams{
			ID:     sch.ID,
			Status: "failed",
			Error:  pgtype.Text{String: err.Error(), Valid: true},
		})
		return
	}

	// Mark as completed
	if err := w.queries.UpdateScheduledEventStatus(ctx, db.UpdateScheduledEventStatusParams{
		ID:     sch.ID,
		Status: "completed",
		Error:  pgtype.Text{Valid: false},
	}); err != nil {
		slog.Error("failed to update scheduled event status",
			"scheduled_id", sch.ID,
			"error", err,
		)
		return
	}

	slog.Info("scheduled event executed",
		"scheduled_id", sch.ID,
		"event_id", event.ID,
		"topic", sch.Topic,
	)
}

// ExecuteNow executes a scheduled event immediately.
// Returns the created event ID.
func (w *Worker) ExecuteNow(ctx context.Context, orgID, scheduleID string) (string, error) {
	// Get the scheduled event with lock
	sch, err := w.queries.GetScheduledEventForExecution(ctx, db.GetScheduledEventForExecutionParams{
		ID:    scheduleID,
		OrgID: orgID,
	})
	if err != nil {
		return "", err
	}

	// Create event from scheduled event
	event := domain.NewEvent(sch.Topic, json.RawMessage(sch.Data))
	event.OrgID = sch.OrgID

	// Publish to NATS
	if err := w.publisher.Publish(ctx, event); err != nil {
		w.queries.UpdateScheduledEventStatus(ctx, db.UpdateScheduledEventStatusParams{
			ID:     sch.ID,
			Status: "failed",
			Error:  pgtype.Text{String: err.Error(), Valid: true},
		})
		return "", err
	}

	// Mark as completed
	if err := w.queries.UpdateScheduledEventStatus(ctx, db.UpdateScheduledEventStatusParams{
		ID:     sch.ID,
		Status: "completed",
		Error:  pgtype.Text{Valid: false},
	}); err != nil {
		slog.Error("failed to update scheduled event status after execution",
			"scheduled_id", sch.ID,
			"error", err,
		)
		// Event was published but status update failed - return success but log error
	}

	slog.Info("scheduled event executed immediately",
		"scheduled_id", sch.ID,
		"event_id", event.ID,
		"topic", sch.Topic,
	)

	return event.ID, nil
}
