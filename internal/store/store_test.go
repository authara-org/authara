package store

import "testing"

func TestPostgreSQLConnectionStringIncludesTLSSettings(t *testing.T) {
	cfg := Config{
		Host:        "postgres.example.com",
		Port:        5432,
		Username:    "authara",
		Password:    "secret",
		Database:    "authara",
		SSLMode:     "verify-full",
		SSLRootCert: "/certs/postgresql-ca.pem",
	}

	want := "host=postgres.example.com port=5432 user=authara password=secret dbname=authara sslmode=verify-full sslrootcert=/certs/postgresql-ca.pem"
	if got := postgreSQLConnectionString(cfg); got != want {
		t.Fatalf("connection string = %q, want %q", got, want)
	}
}
