package email

import "context"

type Service struct {
	sender Sender
}

func NewService(sender Sender) *Service {
	return &Service{sender: sender}
}

func (s *Service) Send(ctx context.Context, to string, msg Message) error {
	return s.sender.Send(ctx, to, msg)
}
