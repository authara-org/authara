package migrations

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDatasourceIncludesTLSSettings(t *testing.T) {
	t.Setenv("POSTGRESQL_HOST", "postgres.example.com")
	t.Setenv("POSTGRESQL_PORT", "5432")
	t.Setenv("POSTGRESQL_DATABASE", "authara")
	t.Setenv("POSTGRESQL_USERNAME", "authara")
	t.Setenv("POSTGRESQL_PASSWORD", "secret")
	t.Setenv("POSTGRESQL_SSL_MODE", "verify-full")
	t.Setenv("POSTGRESQL_SSL_ROOT_CERT", "/certs/postgresql-ca.pem")

	data, err := os.ReadFile("dbconfig.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]struct {
		Datasource string `yaml:"datasource"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	want := "host=postgres.example.com port=5432 dbname=authara user=authara password=secret sslmode=verify-full sslrootcert=/certs/postgresql-ca.pem"
	if got := os.ExpandEnv(config["default"].Datasource); got != want {
		t.Fatalf("datasource = %q, want %q", got, want)
	}
}
