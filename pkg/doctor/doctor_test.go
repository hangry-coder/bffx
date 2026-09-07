package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/version"
	"gopkg.in/yaml.v3"
)

func TestFrameworkVendoringChecks_noGoMod(t *testing.T) {
	root := t.TempDir()
	got := frameworkVendoringChecks(root, nil)
	if len(got) != 0 {
		t.Fatalf("expected no checks without go.mod, got %#v", got)
	}
}

func TestFrameworkVendoringChecks_notVendored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/foo\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := frameworkVendoringChecks(root, nil)
	if len(got) != 1 || got[0].Status != "ok" || !strings.Contains(got[0].Message, "No replace bffx") {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestFrameworkVendoringChecks_vendoredMissingCore(t *testing.T) {
	root := t.TempDir()
	mod := "module example.com/foo\ngo 1.22\n\nrequire bffx v0.0.0\nreplace bffx => ./.bffx/core\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	got := frameworkVendoringChecks(root, nil)
	if len(got) < 1 || got[0].Status != "fail" {
		t.Fatalf("expected fail for missing core, got %#v", got)
	}
}

func TestFrameworkVendoringChecks_staleStamp(t *testing.T) {
	root := t.TempDir()
	mod := "module example.com/foo\ngo 1.22\n\nrequire bffx v0.0.0\nreplace bffx => ./.bffx/core\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".bffx", "core", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".bffx", "framework_version"), []byte("0.0.0-ancient\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := frameworkVendoringChecks(root, nil)
	var sawStamp bool
	for _, r := range got {
		if r.Name == "Framework: version stamp" {
			sawStamp = true
			if r.Status != "warn" {
				t.Fatalf("expected warn on stale stamp, got %#v", r)
			}
			if !strings.Contains(r.Message, version.FrameworkVersion) {
				t.Fatalf("message should mention current framework version: %q", r.Message)
			}
		}
	}
	if !sawStamp {
		t.Fatalf("expected version stamp result, got %#v", got)
	}
}

func TestTLSDoctorChecks(t *testing.T) {
	tests := []struct {
		name       string
		projSpec   string
		envVars    map[string]string
		expectFail []string // Result names we expect to fail
		expectWarn []string
		expectOk   []string
	}{
		{
			name: "Insecure sslmode=disable and redis:// without TLS",
			projSpec: `apiVersion: v1
kind: Project
metadata:
  name: test-project
spec:
  store:
    mode: postgres
    url: postgres://user:pass@localhost:5432/db?sslmode=disable
  runtime:
    redis:
      enabled: true
      url: redis://localhost:6379
`,
			expectFail: []string{"Security: DB TLS", "Security: Redis TLS"},
		},
		{
			name: "Secure baseline",
			projSpec: `apiVersion: v1
kind: Project
metadata:
  name: test-project
spec:
  store:
    mode: postgres
    url: postgres://user:pass@localhost:5432/db?sslmode=require
  runtime:
    redis:
      enabled: true
      url: rediss://localhost:6379
`,
			expectOk: []string{"Security: DB TLS", "Security: Redis TLS"},
		},
		{
			name: "Postgres without sslmode warn",
			projSpec: `apiVersion: v1
kind: Project
metadata:
  name: test-project
spec:
  store:
    mode: postgres
    url: postgres://user:pass@localhost:5432/db
`,
			expectWarn: []string{"Security: DB TLS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("BFFX_ENV", "production")
			defer os.Unsetenv("BFFX_ENV")

			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			dir := t.TempDir()
			bffxDir := filepath.Join(dir, "bffx")
			if err := os.MkdirAll(bffxDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bffxDir, "project.yaml"), []byte(tt.projSpec), 0644); err != nil {
				t.Fatal(err)
			}

			reg, err := manifest.LoadAll(dir)
			if err != nil {
				t.Fatalf("LoadAll failed: %v", err)
			}

			results := tlsSecurityChecks(dir, reg)

			checkResult := func(name string, expectedStatus string) {
				found := false
				for _, r := range results {
					if r.Name == name {
						found = true
						if r.Status != expectedStatus {
							t.Errorf("expected result %q to have status %q, got %q (msg: %s)", name, expectedStatus, r.Status, r.Message)
						}
					}
				}
				if !found && expectedStatus != "" {
					t.Errorf("expected result %q to be present", name)
				}
			}

			for _, name := range tt.expectFail {
				checkResult(name, "fail")
			}
			for _, name := range tt.expectWarn {
				checkResult(name, "warn")
			}
			for _, name := range tt.expectOk {
				checkResult(name, "ok")
			}
		})
	}
}

func TestPipelineDoctorChecks(t *testing.T) {
	reg := &manifest.Registry{}

	projYaml := `
apiVersion: v1
kind: Project
metadata:
  name: test-proj
spec:
  batteries:
    nutrition: openfoodfacts
    vlm: noop
`
	projM := &manifest.Manifest{Kind: "Project"}
	_ = yaml.Unmarshal([]byte(projYaml), &projM)
	reg.Project = projM

	ingPipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: IngestionNoCatalog
spec:
  type: ingestion
  model_routing:
    - gemini
`
	ingM := &manifest.Manifest{Kind: "Pipeline"}
	_ = yaml.Unmarshal([]byte(ingPipelineYaml), &ingM)
	reg.Pipelines = append(reg.Pipelines, ingM)

	got := lintPipelines("", reg)

	var sawDeprecation, sawIngestionFail, sawVlmWarn bool
	for _, r := range got {
		if r.Name == "Pipeline Deprecation" {
			sawDeprecation = true
			if r.Status != "warn" {
				t.Errorf("expected deprecation status to be warn, got %s", r.Status)
			}
		}
		if r.Name == "Pipeline \"IngestionNoCatalog\" Ingestion Check" {
			sawIngestionFail = true
			if r.Status != "fail" {
				t.Errorf("expected ingestion check to fail, got %s", r.Status)
			}
		}
		if r.Name == "Pipeline \"IngestionNoCatalog\" VLM Safety" {
			sawVlmWarn = true
			if r.Status != "warn" {
				t.Errorf("expected VLM safety status to be warn, got %s", r.Status)
			}
		}
	}

	if !sawDeprecation {
		t.Error("expected deprecation check for nutrition battery")
	}
	if !sawIngestionFail {
		t.Error("expected ingestion catalog check failure")
	}
	if !sawVlmWarn {
		t.Error("expected VLM safety warning for mismatch")
	}
}

func TestResourcePolicyDoctorChecks(t *testing.T) {
	os.Setenv("BFFX_ENV", "production")
	defer os.Unsetenv("BFFX_ENV")

	reg := &manifest.Registry{}

	// Case 1: Missing policy block
	res1Yaml := `
apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: MissingPolicyRes
spec:
  fields:
    - { name: name, type: string }
`
	res1 := &manifest.Manifest{Kind: "Resource"}
	res1.Path = "manifests/missing.yaml"
	_ = yaml.Unmarshal([]byte(res1Yaml), &res1)
	reg.Resources = append(reg.Resources, res1)

	// Case 2: Public Write policy in Production
	res2Yaml := `
apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: InsecureRes
spec:
  fields:
    - { name: name, type: string }
  policy:
    read: owner
    write: public
`
	res2 := &manifest.Manifest{Kind: "Resource"}
	res2.Path = "manifests/insecure.yaml"
	_ = yaml.Unmarshal([]byte(res2Yaml), &res2)
	reg.Resources = append(reg.Resources, res2)

	got := lintResourcePolicies(reg)

	var sawOmission, sawInsecure bool
	for _, r := range got {
		if r.Name == "Security: Resource Policy Omission" {
			sawOmission = true
			if r.Status != "fail" {
				t.Errorf("expected omission status to be fail in production, got %s", r.Status)
			}
		}
		if r.Name == "Security: Insecure Public Write Policy" {
			sawInsecure = true
			if r.Status != "fail" {
				t.Errorf("expected write:public to fail in production, got %s", r.Status)
			}
		}
	}

	if !sawOmission {
		t.Error("expected resource policy omission check to trigger")
	}
	if !sawInsecure {
		t.Error("expected insecure public write check to trigger")
	}
}

func TestCheckProductionPostures_RevocationGuardrails(t *testing.T) {
	makeNode := func(y string) yaml.Node {
		var n yaml.Node
		if err := yaml.Unmarshal([]byte(y), &n); err != nil {
			t.Fatalf("unmarshal yaml: %v", err)
		}
		return n
	}
	projectSpec := makeNode(`
runtime:
  redis:
    enabled: false
store:
  mode: postgres
batteries:
  blob: s3
`)
	refreshSpec := makeNode(`
fields:
  - {name: user_id, type: string}
`)

	t.Run("FailsWhenRefreshTokensWithoutRedis", func(t *testing.T) {
		t.Setenv("BFFX_ENV", "production")
		t.Setenv("BFFX_REDIS_ADDR", "")
		t.Setenv("BFFX_JWT_REVOCATION_FAIL_CLOSED", "")

		reg := &manifest.Registry{
			Project: &manifest.Manifest{
				Kind: "Project",
				Spec: projectSpec,
			},
			Resources: []*manifest.Manifest{
				{Kind: "Resource", Metadata: manifest.Metadata{Name: "RefreshToken"}, Spec: refreshSpec},
			},
		}
		got := checkProductionPostures("", reg)
		found := false
		for _, r := range got {
			if r.Name == "Production: JWT Revocation Backend" {
				found = true
				if r.Status != "fail" {
					t.Fatalf("expected fail, got %s", r.Status)
				}
			}
		}
		if !found {
			t.Fatalf("expected revocation backend check, got %#v", got)
		}
	})

	t.Run("FailsWhenFailClosedDisabled", func(t *testing.T) {
		t.Setenv("BFFX_ENV", "production")
		t.Setenv("BFFX_REDIS_ADDR", "127.0.0.1:6379")
		t.Setenv("BFFX_JWT_REVOCATION_FAIL_CLOSED", "0")

		reg := &manifest.Registry{
			Project: &manifest.Manifest{
				Kind: "Project",
				Spec: makeNode(`
runtime:
  redis:
    enabled: true
store:
  mode: postgres
batteries:
  blob: s3
`),
			},
			Resources: []*manifest.Manifest{
				{Kind: "Resource", Metadata: manifest.Metadata{Name: "RefreshToken"}, Spec: refreshSpec},
			},
		}
		got := checkProductionPostures("", reg)
		found := false
		for _, r := range got {
			if r.Name == "Production: JWT Revocation Fail-Closed" {
				found = true
				if r.Status != "fail" {
					t.Fatalf("expected fail, got %s", r.Status)
				}
			}
		}
		if !found {
			t.Fatalf("expected fail-closed check, got %#v", got)
		}
	})
}

func TestDevOnlyRoutesDoctorChecks(t *testing.T) {
	reg := &manifest.Registry{}

	actYaml := `
apiVersion: bffx.io/v1alpha1
kind: Action
metadata:
  name: DevOnlyAct
spec:
  dev_only: true
  route:
    method: POST
    path: /api/v1/dev/reset
    auth: public
`
	act := &manifest.Manifest{Kind: "Action"}
	_ = yaml.Unmarshal([]byte(actYaml), &act)
	reg.Actions = append(reg.Actions, act)

	// In development
	os.Unsetenv("BFFX_ENV")
	gotDev := lintDevOnlyRoutes(reg)
	if len(gotDev) != 1 || gotDev[0].Status != "warn" || !strings.Contains(gotDev[0].Message, "Found dev-only routes") {
		t.Errorf("expected warn status for active dev-only routes in dev, got %#v", gotDev)
	}

	// In production
	os.Setenv("BFFX_ENV", "production")
	defer os.Unsetenv("BFFX_ENV")
	gotProd := lintDevOnlyRoutes(reg)
	if len(gotProd) != 1 || gotProd[0].Status != "ok" || !strings.Contains(gotProd[0].Message, "Dev-only routes are disabled") {
		t.Errorf("expected ok status for disabled dev-only routes in production, got %#v", gotProd)
	}
}

func TestProductionPosturesDoctorChecks(t *testing.T) {
	os.Setenv("BFFX_ENV", "production")
	defer os.Unsetenv("BFFX_ENV")

	reg := &manifest.Registry{}

	projYaml := `
apiVersion: v1
kind: Project
metadata:
  name: test-proj
spec:
  store:
    mode: sqlite
  batteries:
    blob: local
`
	proj := &manifest.Manifest{Kind: "Project"}
	_ = yaml.Unmarshal([]byte(projYaml), &proj)
	reg.Project = proj

	got := checkProductionPostures("", reg)
	var sawSqlite, sawLocalBlob bool
	for _, r := range got {
		if r.Name == "Production: SQLite Store Mode" {
			sawSqlite = true
			if r.Status != "fail" {
				t.Errorf("expected sqlite store check to fail in production, got %s", r.Status)
			}
		}
		if r.Name == "Production: Local Blob Storage" {
			sawLocalBlob = true
			if r.Status != "fail" {
				t.Errorf("expected local blob check to fail in production, got %s", r.Status)
			}
		}
	}

	if !sawSqlite {
		t.Error("expected production sqlite check to trigger")
	}
	if !sawLocalBlob {
		t.Error("expected production local blob check to trigger")
	}
}

func TestBlueprintSecurityDoctorChecks(t *testing.T) {
	reg := &manifest.Registry{}

	bpYaml := `
apiVersion: bffx.io/v1alpha1
kind: Blueprint
metadata:
  name: InitialAdmin
spec:
  resource: AdminUser
  count: 1
  defaults:
    email: admin@example.com
    password: admin123
`
	bp := &manifest.Manifest{Kind: "Blueprint"}
	_ = yaml.Unmarshal([]byte(bpYaml), &bp)
	reg.Blueprints = append(reg.Blueprints, bp)

	// 1. In development, it should be a warning
	os.Unsetenv("BFFX_ENV")
	gotDev := lintBlueprints(reg)
	if len(gotDev) != 1 || gotDev[0].Status != "warn" || !strings.Contains(gotDev[0].Message, "suspicious plaintext credential") {
		t.Errorf("expected warn status for plaintext password in dev blueprint, got %#v", gotDev)
	}

	// 2. In production, it should fail
	os.Setenv("BFFX_ENV", "production")
	defer os.Unsetenv("BFFX_ENV")
	gotProd := lintBlueprints(reg)
	if len(gotProd) != 1 || gotProd[0].Status != "fail" || !strings.Contains(gotProd[0].Message, "suspicious plaintext credential") {
		t.Errorf("expected fail status for plaintext password in prod blueprint, got %#v", gotProd)
	}

	// 3. With env variable reference, it should pass
	bpYamlSecure := `
apiVersion: bffx.io/v1alpha1
kind: Blueprint
metadata:
  name: InitialAdmin
spec:
  resource: AdminUser
  count: 1
  defaults:
    email: admin@example.com
    password: $ADMIN_PASSWORD
`
	bpSecure := &manifest.Manifest{Kind: "Blueprint"}
	_ = yaml.Unmarshal([]byte(bpYamlSecure), &bpSecure)
	reg2 := &manifest.Registry{}
	reg2.Blueprints = append(reg2.Blueprints, bpSecure)

	gotSecure := lintBlueprints(reg2)
	if len(gotSecure) != 0 {
		t.Errorf("expected no warnings for env variable ref, got %#v", gotSecure)
	}
}
