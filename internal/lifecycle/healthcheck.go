package lifecycle

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const healthcheckURL = "http://127.0.0.1:8080/auth/ready"

func RunHealthcheck(ctx context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	return checkHealth(ctx, client, healthcheckURL)
}

func checkHealth(ctx context.Context, client *http.Client, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}
