package proto

import (
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestBuildProto(t *testing.T) {
	// Create mock Resource manifest with CRUD enabled
	var rNode yaml.Node
	_ = yaml.Unmarshal([]byte("{}"), &rNode)
	r := &manifest.Manifest{
		Kind: "Resource",
		Metadata: manifest.Metadata{
			Name: "Meal",
		},
		Spec: rNode,
	}
	rSpec := manifest.ResourceSpec{
		Fields: []manifest.ResourceField{
			{Name: "name", Type: "string"},
			{Name: "calories", Type: "int"},
			{Name: "details", Type: "json"},
			{Name: "logged_at", Type: "datetime"},
		},
	}
	rSpec.Routes.Crud = true
	rData, _ := yaml.Marshal(rSpec)
	_ = yaml.Unmarshal(rData, &r.Spec)

	// Create mock Action manifest with grpc transport only
	var aNode yaml.Node
	_ = yaml.Unmarshal([]byte("{}"), &aNode)
	a := &manifest.Manifest{
		Kind: "Action",
		Metadata: manifest.Metadata{
			Name: "StartFast",
		},
		Spec: aNode,
	}
	aSpec := manifest.ActionSpec{
		Transports: []string{"grpc"},
	}
	aData, _ := yaml.Marshal(aSpec)
	_ = yaml.Unmarshal(aData, &a.Spec)

	// Create mock Action manifest with rest transport only (should be skipped!)
	aRestOnly := &manifest.Manifest{
		Kind: "Action",
		Metadata: manifest.Metadata{
			Name: "SkipMeRest",
		},
		Spec: aNode,
	}
	aRestSpec := manifest.ActionSpec{
		Transports: []string{"rest"},
	}
	aRestData, _ := yaml.Marshal(aRestSpec)
	_ = yaml.Unmarshal(aRestData, &aRestOnly.Spec)

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{r},
		Actions:   []*manifest.Manifest{a, aRestOnly},
	}

	output := BuildProto(reg, "custom.wire.package")

	// 1. Assert Package and syntax
	if !strings.Contains(output, "syntax = \"proto3\";") {
		t.Errorf("Expected proto3 syntax, got:\n%s", output)
	}
	if !strings.Contains(output, "package custom.wire.package;") {
		t.Errorf("Expected custom package name, got:\n%s", output)
	}

	// 2. Assert Dynamic Imports
	if !strings.Contains(output, "import \"google/protobuf/struct.proto\";") {
		t.Errorf("Expected google/protobuf/struct.proto import, got:\n%s", output)
	}
	if !strings.Contains(output, "import \"google/protobuf/timestamp.proto\";") {
		t.Errorf("Expected google/protobuf/timestamp.proto import, got:\n%s", output)
	}

	// 3. Assert CRUD services generated for Meal
	if !strings.Contains(output, "service MealService {") {
		t.Errorf("Expected MealService generation, got:\n%s", output)
	}
	if !strings.Contains(output, "rpc ListMeals(ListMealsRequest) returns (ListMealsResponse);") {
		t.Errorf("Expected ListMeals RPC, got:\n%s", output)
	}

	// 4. Assert ActionService and Transport Gating
	if !strings.Contains(output, "service ActionService {") {
		t.Errorf("Expected ActionService, got:\n%s", output)
	}
	if !strings.Contains(output, "rpc StartFast(StartFastRequest) returns (StartFastResponse);") {
		t.Errorf("Expected StartFast RPC, got:\n%s", output)
	}
	if strings.Contains(output, "SkipMeRest") {
		t.Errorf("Expected SkipMeRest action to be skipped due to transport gating, got:\n%s", output)
	}

	// 5. Assert Field types are mapped correctly
	if !strings.Contains(output, "string name = 2;") {
		t.Errorf("Expected mapped name field, got:\n%s", output)
	}
	if !strings.Contains(output, "int64 calories = 3;") {
		t.Errorf("Expected mapped calories field, got:\n%s", output)
	}
	if !strings.Contains(output, "google.protobuf.Struct details = 4;") {
		t.Errorf("Expected mapped details field, got:\n%s", output)
	}
	if !strings.Contains(output, "google.protobuf.Timestamp logged_at = 5;") {
		t.Errorf("Expected mapped logged_at field, got:\n%s", output)
	}
}

func TestBuildProto_ScreenServices(t *testing.T) {
	// Streaming-enabled screen.
	var sNode yaml.Node
	_ = yaml.Unmarshal([]byte("{}"), &sNode)
	s := &manifest.Manifest{
		Kind: "Screen",
		Metadata: manifest.Metadata{
			Name: "user_profile",
		},
		Spec: sNode,
	}
	sSpec := manifest.ScreenSpec{
		Stream: true,
	}
	sData, _ := yaml.Marshal(sSpec)
	_ = yaml.Unmarshal(sData, &s.Spec)

	// Non-streaming screen.
	var s2Node yaml.Node
	_ = yaml.Unmarshal([]byte("{}"), &s2Node)
	s2 := &manifest.Manifest{
		Kind: "Screen",
		Metadata: manifest.Metadata{
			Name: "settings",
		},
		Spec: s2Node,
	}
	s2Data, _ := yaml.Marshal(manifest.ScreenSpec{})
	_ = yaml.Unmarshal(s2Data, &s2.Spec)

	// REST-only screen (should be skipped from proto).
	var s3Node yaml.Node
	_ = yaml.Unmarshal([]byte("{}"), &s3Node)
	s3 := &manifest.Manifest{
		Kind: "Screen",
		Metadata: manifest.Metadata{
			Name: "rest_only_screen",
		},
		Spec: s3Node,
	}
	s3Data, _ := yaml.Marshal(manifest.ScreenSpec{Transports: []string{"rest"}})
	_ = yaml.Unmarshal(s3Data, &s3.Spec)

	reg := &manifest.Registry{
		Screens: []*manifest.Manifest{s, s2, s3},
	}

	out := BuildProto(reg, "bffx.v1")

	// Streaming screen: both Get and Watch RPCs, with stream keyword.
	if !strings.Contains(out, "service UserProfileScreenService {") {
		t.Errorf("expected strongly-typed UserProfileScreenService, got:\n%s", out)
	}
	if !strings.Contains(out, "rpc Get(UserProfileRequest) returns (UserProfileResponse);") {
		t.Errorf("expected Get RPC, got:\n%s", out)
	}
	if !strings.Contains(out, "rpc Watch(UserProfileRequest) returns (stream UserProfileResponse);") {
		t.Errorf("expected Watch stream RPC for streaming screen, got:\n%s", out)
	}

	// Non-streaming screen: only Get.
	if !strings.Contains(out, "service SettingsScreenService {") {
		t.Errorf("expected SettingsScreenService, got:\n%s", out)
	}
	if !strings.Contains(out, "rpc Get(SettingsRequest) returns (SettingsResponse);") {
		t.Errorf("expected Get RPC for settings, got:\n%s", out)
	}
	if strings.Contains(out, "rpc Watch(SettingsRequest)") {
		t.Errorf("non-streaming screen should not emit Watch, got:\n%s", out)
	}

	// REST-only screen must NOT appear in proto.
	if strings.Contains(out, "RestOnlyScreenService") {
		t.Errorf("REST-only screen should be omitted from proto, got:\n%s", out)
	}

	// Response shape: output (Struct), sections map, version_hash, killed envelope.
	if !strings.Contains(out, "message UserProfileResponse {") {
		t.Errorf("expected UserProfileResponse message, got:\n%s", out)
	}
	if !strings.Contains(out, "google.protobuf.Struct output = 1;") {
		t.Errorf("expected output field on response, got:\n%s", out)
	}
	if !strings.Contains(out, "map<string, google.protobuf.Struct> sections = 2;") {
		t.Errorf("expected sections map field, got:\n%s", out)
	}
	if !strings.Contains(out, "string version_hash = 3;") {
		t.Errorf("expected version_hash field, got:\n%s", out)
	}
	if !strings.Contains(out, "bool killed = 4;") {
		t.Errorf("expected killed field for kill-switch envelope, got:\n%s", out)
	}
}

// TestBuildProto_ScreenServiceTitleStripsScreenSuffix guards the
// screen naming convention (`mobile_screens_home_screen.yaml` ->
// metadata.name = "mobile_screens_home_screen") from emitting the doubled
// `ScreenScreenService` suffix. The generator must prefer `spec.name` and
// strip a trailing `_screen` token from the auto-derived metadata name.
func TestBuildProto_ScreenServiceTitleStripsScreenSuffix(t *testing.T) {
	// 1. Manifest with explicit spec.name should win.
	var explicit yaml.Node
	_ = yaml.Unmarshal([]byte(`
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
output: {}
`), &explicit)
	if explicit.Kind == yaml.DocumentNode && len(explicit.Content) > 0 {
		explicit = *explicit.Content[0]
	}

	// 2. Manifest without spec.name; metadata derived from file path with
	// `_screen` suffix. Generator must drop the suffix.
	var derived yaml.Node
	_ = yaml.Unmarshal([]byte(`
nav_type: bottom
order: 2
route: { method: GET, path: /api/v1/screens/devices }
sources: []
output: {}
`), &derived)
	if derived.Kind == yaml.DocumentNode && len(derived.Content) > 0 {
		derived = *derived.Content[0]
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "MyApp"}},
		Screens: []*manifest.Manifest{
			{Kind: "Screen", Metadata: manifest.Metadata{Name: "mobile_screens_home_screen"}, Spec: explicit},
			{Kind: "Screen", Metadata: manifest.Metadata{Name: "mobile_screens_devices_screen"}, Spec: derived},
		},
	}

	out := BuildProto(reg, "bffx.v1")

	if !strings.Contains(out, "service HomeScreenService {") {
		t.Errorf("explicit spec.name should drive service title: got\n%s", out)
	}
	if strings.Contains(out, "ScreenScreenService") {
		t.Errorf("double-screen suffix should never appear, got:\n%s", out)
	}
	if !strings.Contains(out, "service MobileScreensDevicesScreenService {") {
		t.Errorf("auto-derived name should strip trailing _screen, got:\n%s", out)
	}
}
