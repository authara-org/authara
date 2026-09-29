package roles

import (
	"slices"
	"testing"
)

func TestRoles_AddMethods_Deduplicate(t *testing.T) {
	t.Parallel()

	var rs Roles

	rs.AddAdmin()
	rs.AddAdmin()
	rs.AddOperator()
	rs.AddOperator()

	got := rs.List()
	want := []Role{AutharaAdmin, AutharaOperator}

	if !slices.Equal(got, want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
}

func TestRoles_List_ReturnsClone(t *testing.T) {
	t.Parallel()

	var rs Roles
	rs.AddAdmin()
	rs.AddOperator()

	got := rs.List()
	got[0] = AutharaOperator

	after := rs.List()
	want := []Role{AutharaAdmin, AutharaOperator}

	if !slices.Equal(after, want) {
		t.Fatalf("List() returned non-cloned slice, got %v, want %v", after, want)
	}
}

func TestRoles_Has(t *testing.T) {
	t.Parallel()

	var rs Roles
	rs.AddAdmin()
	rs.AddOperator()

	if !rs.Has(AutharaAdmin) {
		t.Fatal("expected Has(AutharaAdmin) to be true")
	}
	if rs.Has("authara:auditor") {
		t.Fatal("expected removed auditor role to be absent")
	}
	if !rs.Has(AutharaOperator) {
		t.Fatal("expected Has(AutharaOperator) to be true")
	}
}

func TestRoles_HasAny(t *testing.T) {
	t.Parallel()

	var rs Roles
	rs.AddAdmin()

	tests := []struct {
		name    string
		allowed []Role
		want    bool
	}{
		{
			name:    "matching role",
			allowed: []Role{AutharaOperator, AutharaAdmin},
			want:    true,
		},
		{
			name:    "no matching role",
			allowed: []Role{AutharaOperator},
			want:    false,
		},
		{
			name:    "empty allowed list",
			allowed: nil,
			want:    false,
		},
		{
			name:    "duplicate allowed roles",
			allowed: []Role{AutharaAdmin, AutharaAdmin},
			want:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := rs.HasAny(tt.allowed...)
			if got != tt.want {
				t.Fatalf("HasAny(%v) = %v, want %v", tt.allowed, got, tt.want)
			}
		})
	}
}

func TestRoles_IsHelpers(t *testing.T) {
	t.Parallel()

	var rs Roles
	rs.AddAdmin()
	rs.AddOperator()

	if !rs.IsAdmin() {
		t.Fatal("expected IsAdmin() to be true")
	}
	if !rs.IsOperator() {
		t.Fatal("expected IsOperator() to be true")
	}
}

func TestFromClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims []Role
		want   []Role
		err    bool
	}{
		{
			name:   "empty claims",
			claims: nil,
			want:   nil,
			err:    false,
		},
		{
			name:   "single valid claim",
			claims: []Role{AutharaAdmin},
			want:   []Role{AutharaAdmin},
			err:    false,
		},
		{
			name:   "multiple valid claims",
			claims: []Role{AutharaAdmin, AutharaOperator},
			want:   []Role{AutharaAdmin, AutharaOperator},
			err:    false,
		},
		{
			name:   "duplicate claims deduplicated",
			claims: []Role{AutharaAdmin, AutharaAdmin, AutharaOperator},
			want:   []Role{AutharaAdmin, AutharaOperator},
			err:    false,
		},
		{
			name:   "invalid claim rejected",
			claims: []Role{"authara:unknown"},
			want:   nil,
			err:    true,
		},
		{
			name:   "mixed valid and invalid rejected",
			claims: []Role{AutharaAdmin, "authara:unknown"},
			want:   nil,
			err:    true,
		},
		{
			name:   "wrong namespace rejected",
			claims: []Role{"tenant:admin"},
			want:   nil,
			err:    true,
		},
		{
			name:   "empty role rejected",
			claims: []Role{""},
			want:   nil,
			err:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := FromClaims(tt.claims)
			if (err != nil) != tt.err {
				t.Fatalf("FromClaims(%v) error = %v, wantErr %v", tt.claims, err, tt.err)
			}
			if err == nil && !slices.Equal(got.List(), tt.want) {
				t.Fatalf("FromClaims(%v) = %v, want %v", tt.claims, got.List(), tt.want)
			}
		})
	}
}

func TestFromDBRoleNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  []Role
		err   bool
	}{
		{
			name:  "empty role names",
			input: nil,
			want:  nil,
			err:   false,
		},
		{
			name:  "admin maps correctly",
			input: []string{DBAdminRoleName},
			want:  []Role{AutharaAdmin},
			err:   false,
		},
		{
			name:  "operator maps correctly",
			input: []string{DBOperatorRoleName},
			want:  []Role{AutharaOperator},
			err:   false,
		},
		{
			name:  "multiple role names map correctly",
			input: []string{DBAdminRoleName, DBOperatorRoleName},
			want:  []Role{AutharaAdmin, AutharaOperator},
			err:   false,
		},
		{
			name:  "duplicate db role names deduplicated",
			input: []string{DBAdminRoleName, DBAdminRoleName, DBOperatorRoleName},
			want:  []Role{AutharaAdmin, AutharaOperator},
			err:   false,
		},
		{
			name:  "unknown db role rejected",
			input: []string{"owner"},
			want:  nil,
			err:   true,
		},
		{
			name:  "mixed valid and invalid db role rejected",
			input: []string{DBAdminRoleName, "owner"},
			want:  nil,
			err:   true,
		},
		{
			name:  "empty db role rejected",
			input: []string{""},
			want:  nil,
			err:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := FromDBRoleNames(tt.input)
			if (err != nil) != tt.err {
				t.Fatalf("FromDBRoleNames(%v) error = %v, wantErr %v", tt.input, err, tt.err)
			}
			if err == nil && !slices.Equal(got.List(), tt.want) {
				t.Fatalf("FromDBRoleNames(%v) = %v, want %v", tt.input, got.List(), tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role Role
		err  bool
	}{
		{name: "admin valid", role: AutharaAdmin, err: false},
		{name: "operator valid", role: AutharaOperator, err: false},
		{name: "removed auditor invalid", role: "authara:auditor", err: true},
		{name: "removed monitor invalid", role: "authara:monitor", err: true},
		{name: "unknown authara role invalid", role: "authara:unknown", err: true},
		{name: "wrong namespace invalid", role: "tenant:admin", err: true},
		{name: "empty invalid", role: "", err: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate(tt.role)
			if (err != nil) != tt.err {
				t.Fatalf("validate(%q) error = %v, wantErr %v", tt.role, err, tt.err)
			}
		})
	}
}
