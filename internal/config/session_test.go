package config

import (
	"testing"
	"time"
)

func TestSessionParseRecentAuthenticationWindow(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "minimum", value: "1m", want: time.Minute},
		{name: "normal", value: "10m", want: 10 * time.Minute},
		{name: "maximum", value: "1h", want: time.Hour},
		{name: "too short", value: "59s", wantErr: true},
		{name: "too long", value: "1h1s", wantErr: true},
		{name: "invalid", value: "later", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := Session{RefreshTokenRotationRaw: "24h", RecentAuthenticationWindowRaw: test.value}
			err := cfg.parse()
			if test.wantErr {
				if err == nil {
					t.Fatal("expected parse to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.RecentAuthenticationWindow != test.want {
				t.Fatalf("window = %s, want %s", cfg.RecentAuthenticationWindow, test.want)
			}
		})
	}
}
