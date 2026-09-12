package bootstrap

import (
	"strings"
	"testing"
)

func TestNewServicesRequiresConfigService(t *testing.T) {
	for _, test := range []struct {
		name string
		app  *App
		want string
	}{
		{name: "app", want: "app is required"},
		{name: "config service", app: &App{}, want: "config service is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewServices(test.app)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewServices error = %v, want %q", err, test.want)
			}
		})
	}
}
