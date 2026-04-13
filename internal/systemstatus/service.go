package systemstatus

import "context"

type Checker interface {
	CheckHealth(ctx context.Context) error
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Snapshot struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

type Service struct {
	EnableDatabase bool
	RequireWorker  bool
	Database       Checker
	Worker         Checker
}

func (s Service) SystemStatus(ctx context.Context) (Snapshot, error) {
	checks := []Check{
		s.checkDatabase(ctx),
		s.checkWorker(ctx),
	}

	status := "ready"
	for _, item := range checks {
		if item.Status == "not_ready" {
			status = "not_ready"
			break
		}
	}

	return Snapshot{
		Status: status,
		Checks: checks,
	}, nil
}

func (s Service) checkDatabase(ctx context.Context) Check {
	if !s.EnableDatabase {
		return Check{
			Name:    "database",
			Status:  "disabled",
			Message: "database disabled",
		}
	}
	if s.Database == nil {
		return Check{
			Name:    "database",
			Status:  "not_ready",
			Message: "database checker unavailable",
		}
	}
	if err := s.Database.CheckHealth(ctx); err != nil {
		return Check{
			Name:    "database",
			Status:  "not_ready",
			Message: err.Error(),
		}
	}
	return Check{Name: "database", Status: "ready"}
}

func (s Service) checkWorker(ctx context.Context) Check {
	if !s.RequireWorker {
		return Check{
			Name:    "worker",
			Status:  "disabled",
			Message: "worker disabled",
		}
	}
	if s.Worker == nil {
		return Check{
			Name:    "worker",
			Status:  "not_ready",
			Message: "worker checker unavailable",
		}
	}
	if err := s.Worker.CheckHealth(ctx); err != nil {
		return Check{
			Name:    "worker",
			Status:  "not_ready",
			Message: err.Error(),
		}
	}
	return Check{Name: "worker", Status: "ready"}
}
