package config

type Observability struct {
	Enabled bool `env:"AUTHARA_METRICS_ENABLED,default=false"`
}
