package docs

import (
	"os"
	"strings"
	"testing"
)

func TestOfficialComposeExamplesWaitForMigrations(t *testing.T) {
	required := []string{
		`test: ["CMD-SHELL", "pg_isready -U authara -d authara"]`,
		`POSTGRESQL_HOST: postgres`,
		`command: ["up", "-env=default", "-config=/migrations/dbconfig.yaml"]`,
		`condition: service_healthy`,
		`condition: service_completed_successfully`,
	}

	for _, path := range []string{"../README.md", "quickstart.md", "deployment/docker.md"} {
		contents := readDocumentation(t, path)
		for _, value := range required {
			if !strings.Contains(contents, value) {
				t.Errorf("%s is missing %q", path, value)
			}
		}
	}
}

func TestOfficialDockerCommandsRunMigrationsUp(t *testing.T) {
	for path, minimum := range map[string]int{
		"quickstart.md":            1,
		"operations/migrations.md": 1,
	} {
		contents := readDocumentation(t, path)
		if count := strings.Count(contents, "up -env=default -config=/migrations/dbconfig.yaml"); count < minimum {
			t.Errorf("%s contains %d explicit migration commands, want at least %d", path, count, minimum)
		}
	}
}

func readDocumentation(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
