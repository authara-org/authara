package webhook

import "context"

type NoopPublisher struct{}

func (NoopPublisher) Publish(ctx context.Context, evt Envelope) error {
	return nil
}
