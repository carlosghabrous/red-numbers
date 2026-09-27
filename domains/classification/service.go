package classification

import "context"

// Service coordinates classification log retrieval for the router.
type Service struct {
	logs *Repository
}

// NewService creates a classification service backed by a log repository.
func NewService(logs *Repository) *Service {
	return &Service{logs: logs}
}

// GetLogs returns recent automatic classification decisions.
func (s *Service) GetLogs(ctx context.Context) ([]LogEntry, error) {
	return s.logs.GetLogs(ctx)
}
