package config

import "testing"

func TestDBValidateSSLMode(t *testing.T) {
	for _, mode := range []string{"disable", "require", "verify-ca", "verify-full"} {
		db := validTestDB()
		db.SSLMode = mode
		if err := db.validate(); err != nil {
			t.Errorf("mode %q: %v", mode, err)
		}
	}

	db := validTestDB()
	db.SSLMode = "prefer"
	if err := db.validate(); err == nil {
		t.Fatal("expected unsupported SSL mode to fail")
	}
}

func validTestDB() DB {
	return DB{
		Host:         "postgres",
		Port:         5432,
		Database:     "authara",
		Timezone:     "UTC",
		SSLMode:      "disable",
		MaxOpenConns: 1,
	}
}
