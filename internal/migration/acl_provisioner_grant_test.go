package migration_test

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-ANUJDS: unmatched_principal: provision creates a stub user entity under
// system:provisioner. Without a create grant on the user type, that create is
// ACL-denied and provision never works; this migration injects the minimal
// grant. It only touches a policy that actually opts into provision.

const provisionLookup = "user_entity_type: person\nprincipal_property: sub\n"

func TestACLProvisionerGrant_Detect(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "provision policy, provisioner ungranted",
			src:  provisionLookup + "unmatched_principal: provision\nroles:\n  viewer:\n    read: [ticket]\n",
			want: true,
		},
		{
			name: "provision policy, provisioner already granted create",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  provisioner-system:\n    create: [person]\n" +
				"assignments:\n  system:provisioner: provisioner-system\n",
			want: false,
		},
		{
			name: "provision policy, provisioner granted via operator's own role",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  onboard:\n    create: [person]\n" +
				"assignments:\n  system:provisioner: onboard\n",
			want: false,
		},
		{
			name: "not a provision policy (reject)",
			src:  provisionLookup + "unmatched_principal: reject\nroles:\n  viewer:\n    read: [ticket]\n",
			want: false,
		},
		{
			name: "not a provision policy (absent)",
			src:  "roles:\n  viewer:\n    read: [ticket]\n",
			want: false,
		},
		{
			name: "empty document",
			src:  "",
			want: false,
		},
		{
			name: "assignment names a role that is not defined",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  viewer:\n    read: [ticket]\nassignments:\n  system:provisioner: ghost\n",
			want: true,
		},
		{
			name: "assigned role grants create on the WRONG type",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  writer:\n    create: [ticket]\n" +
				"assignments:\n  system:provisioner: writer\n",
			want: true,
		},
		{
			name: "granted create via wildcard",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  super:\n    create: [\"*\"]\n" +
				"assignments:\n  system:provisioner: super\n",
			want: false,
		},
		{
			name: "granted via asserted_role_assignments (scalar)",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  onboard:\n    create: [person]\n" +
				"asserted_role_assignments:\n  system:provisioner: onboard\n",
			want: false,
		},
		{
			name: "granted via asserted_role_assignments (list)",
			src: provisionLookup + "unmatched_principal: provision\n" +
				"roles:\n  onboard:\n    create: [person]\n" +
				"asserted_role_assignments:\n  system:provisioner: [other, onboard]\n",
			want: false,
		},
	}

	m := &migration.ACLProvisionerGrantMigration{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.Detect(parseDoc(t, tc.src)); got != tc.want {
				t.Errorf("Detect() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestACLProvisionerGrant_ApplyIsIdempotent: after Apply, Detect must be false
// and a second Apply must not change the document.
func TestACLProvisionerGrant_ApplyIsIdempotent(t *testing.T) {
	sources := map[string]string{
		"well formed": provisionLookup + "unmatched_principal: provision\n" +
			"roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n",
		"roles null": provisionLookup + "unmatched_principal: provision\nroles:\nassignments:\n  alice: viewer\n",
		"assignments null": provisionLookup + "unmatched_principal: provision\n" +
			"roles:\n  viewer:\n    read: [ticket]\nassignments:\n",
		"both null": provisionLookup + "unmatched_principal: provision\nroles:\nassignments:\n",
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			m := &migration.ACLProvisionerGrantMigration{}
			doc := parseDoc(t, src)
			if err := m.Apply(doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if m.Detect(doc) {
				out, _ := yaml.Marshal(doc)
				t.Errorf("Detect still true after Apply — migration never converges:\n%s", out)
			}
			first, _ := yaml.Marshal(doc)
			if err := m.Apply(doc); err != nil {
				t.Fatalf("second Apply: %v", err)
			}
			second, _ := yaml.Marshal(doc)
			if !bytes.Equal(first, second) {
				t.Errorf("second Apply changed the document:\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// TestACLProvisionerGrant_ProducesLoadablePolicy: the migrated policy must
// parse, Validate, and actually bind the provisioner to a role that creates the
// user type.
func TestACLProvisionerGrant_ProducesLoadablePolicy(t *testing.T) {
	sources := map[string]string{
		"with existing roles": provisionLookup + "unmatched_principal: provision\n" +
			"roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n",
		"roles only": provisionLookup + "unmatched_principal: provision\nroles:\n  viewer:\n    read: [ticket]\n",
		"roles null": provisionLookup + "unmatched_principal: provision\nroles:\nassignments:\n  alice: viewer\n",
		"dangling assignment": provisionLookup + "unmatched_principal: provision\n" +
			"roles:\n  viewer:\n    read: [ticket]\nassignments:\n  system:provisioner: ghost\n",
		"assigned role creates wrong type": provisionLookup + "unmatched_principal: provision\n" +
			"roles:\n  writer:\n    create: [ticket]\nassignments:\n  system:provisioner: writer\n",
	}

	m := &migration.ACLProvisionerGrantMigration{}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			doc := parseDoc(t, src)
			if err := m.Apply(doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			out, err := yaml.Marshal(doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var policy acl.Policy
			if err := yaml.Unmarshal(out, &policy); err != nil {
				t.Fatalf("migrated policy does not parse: %v\n%s", err, out)
			}
			if err := policy.Validate(); err != nil {
				t.Fatalf("migrated policy fails Validate (would block boot): %v\n%s", err, out)
			}

			role, ok := policy.Assignments[principal.UserProvisioner]
			if !ok {
				t.Fatalf("no assignment for %q:\n%s", principal.UserProvisioner, out)
			}
			def, ok := policy.Roles[role]
			if !ok {
				t.Fatalf("assignment names role %q which is not defined:\n%s", role, out)
			}
			if !containsCreate(def.Create, policy.UserEntityType) {
				t.Errorf("role %q does not grant create on %q: create=%v\n%s",
					role, policy.UserEntityType, def.Create, out)
			}
			// A role WE mint must be create-only on the user type — never any
			// read/update/delete, and never another type (the bare-stub
			// containment, RR-28SCW3). A role the operator wrote is theirs.
			if role == "provisioner-system" {
				if len(def.Read)+len(def.Update)+len(def.Delete) != 0 {
					t.Errorf("migration-created role grants verbs beyond create "+
						"(read=%v update=%v delete=%v)", def.Read, def.Update, def.Delete)
				}
				if len(def.Create) != 1 || def.Create[0] != policy.UserEntityType {
					t.Errorf("migration-created role creates more than the user type: %v", def.Create)
				}
			}
		})
	}
}

func containsCreate(create []string, userType string) bool {
	for _, c := range create {
		if c == "*" || c == userType {
			return true
		}
	}
	return false
}

// TestACLProvisionerGrant_RespectsOperatorRole: if the operator already defines
// a role by our name, bind to theirs rather than overwriting it.
func TestACLProvisionerGrant_RespectsOperatorRole(t *testing.T) {
	src := provisionLookup + "unmatched_principal: provision\n" +
		"roles:\n  provisioner-system:\n    create: [person, ticket]\nassignments:\n  alice: viewer\n"
	m := &migration.ACLProvisionerGrantMigration{}
	doc := parseDoc(t, src)
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, _ := yaml.Marshal(doc)
	var policy acl.Policy
	if err := yaml.Unmarshal(out, &policy); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Their wider definition survives; we did not narrow it.
	got := policy.Roles["provisioner-system"].Create
	if len(got) != 2 {
		t.Errorf("operator's role was overwritten: create = %v, want [person ticket]", got)
	}
	if policy.Assignments[principal.UserProvisioner] != "provisioner-system" {
		t.Errorf("assignment = %q, want provisioner-system", policy.Assignments[principal.UserProvisioner])
	}
}

// TestACLProvisionerGrant_LeavesNonProvisionPolicyAlone: a reject/anonymous
// policy must be byte-for-byte untouched.
func TestACLProvisionerGrant_LeavesNonProvisionPolicyAlone(t *testing.T) {
	src := provisionLookup + "unmatched_principal: reject\nroles:\n  viewer:\n    read: [ticket]\n"
	m := &migration.ACLProvisionerGrantMigration{}
	doc := parseDoc(t, src)
	if m.Detect(doc) {
		t.Fatal("Detect true on a non-provision policy — would inject an unwanted grant")
	}
	before, _ := yaml.Marshal(doc)
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after, _ := yaml.Marshal(doc)
	if !bytes.Equal(before, after) {
		t.Errorf("Apply mutated a non-provision policy:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// TestACLProvisionerGrant_PreservesComments keeps an operator's file readable.
func TestACLProvisionerGrant_PreservesComments(t *testing.T) {
	src := `# Our access policy.
user_entity_type: person
principal_property: sub
unmatched_principal: provision
roles:
  # Analysts get read-only access.
  viewer:
    read: [ticket]
assignments:
  alice: viewer
`
	m := &migration.ACLProvisionerGrantMigration{}
	doc := parseDoc(t, src)
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, _ := yaml.Marshal(doc)
	got := string(out)
	for _, want := range []string{
		"# Our access policy.",
		"# Analysts get read-only access.",
		"alice: viewer",
		"provisioner-system",
		"system:provisioner",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

// TestACLProvisionerGrant_PrincipalMatchesRuntime pins the literal in this
// package against the constant provisioning actually stamps. internal/migration
// deliberately imports nothing but yaml, so the value is duplicated — this test
// keeps the copy honest.
func TestACLProvisionerGrant_PrincipalMatchesRuntime(t *testing.T) {
	if migration.ProvisionerPrincipal != principal.UserProvisioner {
		t.Fatalf("migration.ProvisionerPrincipal = %q but principal.UserProvisioner = %q; "+
			"the migration would grant a principal nothing provisions as",
			migration.ProvisionerPrincipal, principal.UserProvisioner)
	}
}

// TestACLProvisionerGrant_ApplyFailsLoudlyWhenItCannotGrant: the input is a
// provision policy whose provisioner assignment value is a nested mapping, not a
// role name — the repair cannot fix it, and Apply must error rather than persist
// a file that grants nothing.
func TestACLProvisionerGrant_ApplyFailsLoudlyWhenItCannotGrant(t *testing.T) {
	src := provisionLookup + "unmatched_principal: provision\n" +
		"roles:\n  viewer:\n    read: [ticket]\n" +
		"assignments:\n  system:provisioner:\n    nested: yes\n"
	m := &migration.ACLProvisionerGrantMigration{}
	doc := parseDoc(t, src)

	err := m.Apply(doc)
	if err == nil {
		out, _ := yaml.Marshal(doc)
		t.Fatalf("Apply reported success but granted nothing:\n%s", out)
	}
	if !strings.Contains(err.Error(), migration.ProvisionerPrincipal) {
		t.Errorf("error should name the principal, got: %v", err)
	}
}

// TestACLProvisionerGrant_Registered pins the wiring.
func TestACLProvisionerGrant_Registered(t *testing.T) {
	for _, m := range migration.ForFileType(migration.FileTypeACL) {
		if m.Name() == "acl-provisioner-grant" {
			return
		}
	}
	t.Fatal("acl-provisioner-grant is not registered for FileTypeACL")
}
