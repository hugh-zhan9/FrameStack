package workerclient

import "context"

type HealthChecker struct {
	Client Client
}

func (h HealthChecker) CheckWorkerHealth(ctx context.Context) error {
	session, err := h.Client.Start(ctx)
	if err != nil {
		return err
	}
	defer session.Close()

	_, err = session.HealthCheck(ctx)
	return err
}

func (h HealthChecker) CheckHealth(ctx context.Context) error {
	return h.CheckWorkerHealth(ctx)
}
