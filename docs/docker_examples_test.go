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

func TestOfficialComposeExamplesWaitForCoreReadiness(t *testing.T) {
	required := "authara:\n        condition: service_healthy"
	for _, path := range []string{"../README.md", "quickstart.md", "deployment/docker.md"} {
		if contents := readDocumentation(t, path); !strings.Contains(contents, required) {
			t.Errorf("%s does not gate gateway startup on Core readiness", path)
		}
	}
}

func TestKubernetesProbeExampleSeparatesLivenessAndReadiness(t *testing.T) {
	contents := readDocumentation(t, "operations/healthchecks.md")
	for _, required := range []string{
		"startupProbe:",
		"livenessProbe:",
		"readinessProbe:",
		"path: /auth/live",
		"path: /auth/ready",
	} {
		if !strings.Contains(contents, required) {
			t.Errorf("operations/healthchecks.md is missing %q", required)
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
