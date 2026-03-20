package webhook

import "context"

type FilteringPublisher struct {
	Inner   Publisher
	Enabled map[string]struct{}
}

func NewFilteringPublisher(inner Publisher, enabled map[string]struct{}) *FilteringPublisher {
	return &FilteringPublisher{
		Inner:   inner,
		Enabled: enabled,
	}
}

func (p *FilteringPublisher) Publish(ctx context.Context, evt Envelope) error {
	// empty = allow all
	if len(p.Enabled) == 0 {
		return p.Inner.Publish(ctx, evt)
	}

	if _, ok := p.Enabled[string(evt.Type)]; !ok {
		return nil
	}

	return p.Inner.Publish(ctx, evt)
}
