package messaging

import "context"

type eventIDContextKey struct{}

// WithEventID stores the envelope event id for handlers that deduplicate a delivery.
func WithEventID(ctx context.Context, eventID string) context.Context {
	if eventID == "" {
		return ctx
	}
	return context.WithValue(ctx, eventIDContextKey{}, eventID)
}

// EventIDFromContext returns the envelope event id set by the consumer.
func EventIDFromContext(ctx context.Context) string {
	eventID, _ := ctx.Value(eventIDContextKey{}).(string)
	return eventID
}
