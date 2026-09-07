package mobile

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"gopkg.in/yaml.v3"
)

func TestGenerateFlutterClient(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flutter-gen-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "testapp"},
		},
	}
	
	spec := manifest.ProjectSpec{}
	spec.App.ApiPrefix = "/api/v1"
	spec.App.AuthStrategy = "optional"
	spec.App.MinClientVersion = "1.2.3"
	spec.Runtime.Api.Port = 8080
	
	specBytes, _ := yaml.Marshal(spec)
	yaml.Unmarshal(specBytes, &reg.Project.Spec)

	// Inject a mock action with path parameters to verify Task 2.3
	actionManifest := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "UpdateRoutine"},
	}
	actionSpec := manifest.ActionSpec{}
	actionSpec.Route.Path = "/api/v1/app/routines/{id}"
	actionSpec.Route.Method = "POST"
	actionBytes, _ := yaml.Marshal(actionSpec)
	yaml.Unmarshal(actionBytes, &actionManifest.Spec)
	reg.Actions = append(reg.Actions, actionManifest)

	err = GenerateFlutterClient(tmpDir, reg, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	// Check bffx_config.gen.dart
	configPath := filepath.Join(tmpDir, "bffx_config.gen.dart")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config := string(configBytes)
	if !strings.Contains(config, "static const String clientVersion = '1.2.3';") {
		t.Errorf("missing clientVersion in config")
	}

	// Check bffx_client.gen.dart
	clientPath := filepath.Join(tmpDir, "bffx_client.gen.dart")
	clientBytes, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	client := string(clientBytes)
	
	// Check for fixed headers
	if !strings.Contains(client, "if (_deviceId != null) 'X-Device-ID': _deviceId!") {
		t.Errorf("missing X-Device-ID with if condition")
	}
	if !strings.Contains(client, "if (BFFXConfig.appSecret.isNotEmpty) 'X-App-Secret': BFFXConfig.appSecret") {
		t.Errorf("missing X-App-Secret in headers")
	}
	
	// Check for PATCH support
	if !strings.Contains(client, "case 'PATCH':") || !strings.Contains(client, "return http.patch(uri") {
		t.Errorf("missing PATCH support in request method")
	}

	// Auth helpers must route through request() so lifecycle headers are sent
	if !strings.Contains(client, "await request('/auth/anonymous'") {
		t.Errorf("createAnonymousSession should use request() for X-BFFX-Client-Version")
	}
	if strings.Contains(client, "Uri.parse('${BFFXConfig.baseUrl}/auth/anonymous')") {
		t.Errorf("auth helpers should not bypass request() with raw Uri.parse")
	}

	// Check path parameter generation (Task 2.3)
	if !strings.Contains(client, "Future<Map<String, dynamic>> postUpdateRoutine(String id, Map<String, dynamic> body") {
		t.Errorf("missing path parameter in generated method signature: %s", client)
	}
	if !strings.Contains(client, "request('/app/routines/$id'") {
		t.Errorf("missing interpolated path parameter in request call: %s", client)
	}
}

func TestGenerateFlutterClient_WithGrpcEnabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flutter-gen-grpc-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "playgame"},
		},
	}
	
	spec := manifest.ProjectSpec{}
	spec.App.ApiPrefix = "/api/v1"
	spec.Runtime.Api.Port = 8080
	spec.Runtime.Wire.Enabled = true
	spec.Runtime.Wire.GRPCPort = 9555
	
	specBytes, _ := yaml.Marshal(spec)
	yaml.Unmarshal(specBytes, &reg.Project.Spec)

	err = GenerateFlutterClient(tmpDir, reg, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	// 1. Check bffx_config.gen.dart for gRPC properties
	configPath := filepath.Join(tmpDir, "bffx_config.gen.dart")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config := string(configBytes)
	if !strings.Contains(config, "static const bool grpcEnabled = true;") {
		t.Errorf("missing grpcEnabled in config")
	}
	if !strings.Contains(config, "static String get grpcHost") {
		t.Errorf("missing grpcHost getter in config")
	}
	if !strings.Contains(config, "static const int grpcPort = 9555;") {
		t.Errorf("missing grpcPort setting in config: %s", config)
	}

	// 2. Check bffx_client.gen.dart for package:grpc import and getters
	clientPath := filepath.Join(tmpDir, "bffx_client.gen.dart")
	clientBytes, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatal(err)
	}
	client := string(clientBytes)
	if !strings.Contains(client, "import 'package:grpc/grpc.dart';") {
		t.Errorf("missing package:grpc import in client")
	}
	if !strings.Contains(client, "ClientChannel get grpcChannel") {
		t.Errorf("missing grpcChannel getter in client")
	}
	if !strings.Contains(client, "CallOptions get grpcOptions") {
		t.Errorf("missing grpcOptions getter in client: %s", client)
	}
}

func TestGenerateFlutterClient_DartScreenGRPCFacade(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flutter-gen-grpc-screen")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "myapp"},
		},
	}
	spec := manifest.ProjectSpec{}
	spec.App.ApiPrefix = "/api/v1"
	spec.Runtime.Api.Port = 8080
	spec.Runtime.Wire.Enabled = true
	spec.Runtime.Wire.GRPCPort = 9595
	specBytes, _ := yaml.Marshal(spec)
	_ = yaml.Unmarshal(specBytes, &reg.Project.Spec)

	// Add a streaming screen and a non-grpc-transport screen.
	screen := &manifest.Manifest{
		Kind:     "Screen",
		Metadata: manifest.Metadata{Name: "user_profile"},
	}
	sSpec := manifest.ScreenSpec{Stream: true}
	sData, _ := yaml.Marshal(sSpec)
	_ = yaml.Unmarshal(sData, &screen.Spec)
	reg.Screens = append(reg.Screens, screen)

	restOnly := &manifest.Manifest{
		Kind:     "Screen",
		Metadata: manifest.Metadata{Name: "rest_only"},
	}
	rSpec := manifest.ScreenSpec{Transports: []string{"rest"}}
	rData, _ := yaml.Marshal(rSpec)
	_ = yaml.Unmarshal(rData, &restOnly.Spec)
	reg.Screens = append(reg.Screens, restOnly)

	if err := GenerateFlutterClient(tmpDir, reg, tmpDir); err != nil {
		t.Fatalf("generate: %v", err)
	}

	grpcPath := filepath.Join(tmpDir, "bffx_grpc_client.gen.dart")
	bytes, err := os.ReadFile(grpcPath)
	if err != nil {
		t.Fatalf("expected gRPC facade to be emitted: %v", err)
	}
	out := string(bytes)

	if !strings.Contains(out, "class BFFXScreenGRPCClient") {
		t.Errorf("missing facade class:\n%s", out)
	}
	if !strings.Contains(out, "UserProfileScreenServiceClient userProfile;") {
		t.Errorf("missing strongly-typed screen client field:\n%s", out)
	}
	if strings.Contains(out, "RestOnlyScreen") {
		t.Errorf("rest-only screen should not appear in gRPC facade:\n%s", out)
	}
	if !strings.Contains(out, "ChannelOptions(credentials: ChannelCredentials.insecure())") {
		t.Errorf("missing channel construction:\n%s", out)
	}
	if !strings.Contains(out, "metadata['x-app-secret'] = BFFXConfig.appSecret") {
		t.Errorf("missing x-app-secret in gRPC CallOptions:\n%s", out)
	}
}

func TestGenerateFlutterClient_NoFacadeWhenGRPCDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flutter-gen-no-grpc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "demo"}},
	}
	spec := manifest.ProjectSpec{}
	spec.App.ApiPrefix = "/api/v1"
	spec.Runtime.Wire.Enabled = false
	specBytes, _ := yaml.Marshal(spec)
	_ = yaml.Unmarshal(specBytes, &reg.Project.Spec)

	if err := GenerateFlutterClient(tmpDir, reg, tmpDir); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "bffx_grpc_client.gen.dart")); err == nil {
		t.Fatal("gRPC facade should NOT be emitted when wire disabled")
	}
}
