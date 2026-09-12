package webhook

import "context"

type FilteringPublisher struct {
	Inner   Publisher
	Enabled map[string]struct{}
	Policy  func() []string
}

func NewFilteringPublisher(inner Publisher, enabled map[string]struct{}) *FilteringPublisher {
	return &FilteringPublisher{
		Inner:   inner,
		Enabled: enabled,
	}
}

func NewFilteringPublisherWithPolicy(inner Publisher, policy func() []string) *FilteringPublisher {
	return &FilteringPublisher{Inner: inner, Policy: policy}
}

func (p *FilteringPublisher) Publish(ctx context.Context, evt Envelope) error {
	if p.Policy != nil {
		enabled := p.Policy()
		if len(enabled) == 0 {
			return p.Inner.Publish(ctx, evt)
		}
		for _, event := range enabled {
			if event == string(evt.Type) {
				return p.Inner.Publish(ctx, evt)
			}
		}
		return nil
	}
	// empty = allow all
	if len(p.Enabled) == 0 {
		return p.Inner.Publish(ctx, evt)
	}

	if _, ok := p.Enabled[string(evt.Type)]; !ok {
		return nil
	}

	return p.Inner.Publish(ctx, evt)
}
