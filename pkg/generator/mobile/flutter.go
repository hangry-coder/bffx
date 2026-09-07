package mobile

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var pathParamRegex = regexp.MustCompile(`{([a-zA-Z0-9_]+)}`)

func parsePathParams(path string) (args []string, dartPath string) {
	matches := pathParamRegex.FindAllStringSubmatch(path, -1)
	dartPath = path
	for _, match := range matches {
		original := match[0]
		name := match[1]
		camelName := toIdiomaticMethodName(name, "")
		if camelName == "" {
			camelName = name
		}
		camelName = toCamelCase(camelName)
		if len(camelName) > 0 {
			camelName = strings.ToLower(camelName[:1]) + camelName[1:]
		}
		args = append(args, "String "+camelName)
		dartPath = strings.ReplaceAll(dartPath, original, "$"+camelName)
	}
	return args, dartPath
}


func GenerateFlutterClient(root string, reg *manifest.Registry, outputDir string) error {
	clientDir := outputDir
	if clientDir == "" {
		clientDir = filepath.Join(root, "clients", "flutter", "lib")
	}
	os.MkdirAll(clientDir, 0o755)

	proj := reg.ProjectSpec()
	authStrategy := proj.App.AuthStrategy
	if authStrategy == "" {
		authStrategy = "optional"
	}

	// 1. Generate bffx_config.gen.dart
	var configSb strings.Builder
	configSb.WriteString("import 'dart:io' show Platform;\n")
	configSb.WriteString("import 'package:flutter/foundation.dart' show kReleaseMode;\n\n")
	configSb.WriteString("class BFFXConfig {\n")
	configSb.WriteString("  static String get baseUrl {\n")
	configSb.WriteString("    // Enforce HTTPS in release mode\n")
	configSb.WriteString("    if (kReleaseMode) {\n")
	configSb.WriteString(fmt.Sprintf("      return 'https://api.%s%s'; // TODO: Update with your production domain\n", reg.Project.Metadata.Name, proj.App.ApiPrefix))
	configSb.WriteString("    }\n\n")
	configSb.WriteString("    try {\n")
	configSb.WriteString(fmt.Sprintf("      if (Platform.isAndroid) return 'http://10.0.2.2:%d%s';\n", proj.Runtime.Api.Port, proj.App.ApiPrefix))
	configSb.WriteString("    } catch (_) {}\n")
	configSb.WriteString(fmt.Sprintf("    return 'http://localhost:%d%s';\n", proj.Runtime.Api.Port, proj.App.ApiPrefix))
	configSb.WriteString("  }\n")
	configSb.WriteString(fmt.Sprintf("  static const String authStrategy = '%s';\n", authStrategy))
	configSb.WriteString("  static const String appSecret = ''; // To be injected via build-arg\n")
	clientVersion := proj.App.MinClientVersion
	if clientVersion == "" {
		clientVersion = "1.0.0"
	}
	configSb.WriteString(fmt.Sprintf("  static const String clientVersion = '%s';\n", clientVersion))
	configSb.WriteString(fmt.Sprintf("  static const bool idempotencyEnabled = %t;\n", proj.FeatureFlags.Idempotency))
	configSb.WriteString(fmt.Sprintf("  static const bool grpcEnabled = %t;\n", proj.Runtime.Wire.Enabled))
	if proj.Runtime.Wire.Enabled {
		configSb.WriteString("  static String get grpcHost {\n")
		configSb.WriteString("    if (kReleaseMode) {\n")
		configSb.WriteString(fmt.Sprintf("      return 'api.%s.com';\n", reg.Project.Metadata.Name))
		configSb.WriteString("    }\n")
		configSb.WriteString("    try {\n")
		configSb.WriteString("      if (Platform.isAndroid) return '10.0.2.2';\n")
		configSb.WriteString("    } catch (_) {}\n")
		configSb.WriteString("    return 'localhost';\n")
		configSb.WriteString("  }\n")
		grpcPort := proj.Runtime.Wire.GRPCPort
		if grpcPort == 0 {
			grpcPort = 9090
		}
		configSb.WriteString(fmt.Sprintf("  static const int grpcPort = %d;\n", grpcPort))
	}
	configSb.WriteString("\n")
	configSb.WriteString("  static Future<void> ensureInitialized() async {\n")
	configSb.WriteString("    // Placeholder for async initialization logic\n")
	configSb.WriteString("  }\n")
	configSb.WriteString("}\n")
	
	if err := os.WriteFile(filepath.Join(clientDir, "bffx_config.gen.dart"), []byte(configSb.String()), 0o644); err != nil {
		return err
	}

	// 2. Generate bffx_client.gen.dart
	apiPrefix := proj.App.ApiPrefix
	if apiPrefix == "" {
		apiPrefix = "/api/v1"
	}

	var sb strings.Builder
	sb.WriteString("// GENERATED CODE - DO NOT MODIFY BY HAND\n\n")
	sb.WriteString("import 'dart:convert';\n")
	sb.WriteString("import 'dart:io' show Platform;\n")
	sb.WriteString("import 'package:http/http.dart' as http;\n")
	sb.WriteString("import 'package:uuid/uuid.dart';\n")
	if proj.Runtime.Wire.Enabled {
		sb.WriteString("import 'package:grpc/grpc.dart';\n")
	}
	sb.WriteString("import 'bffx_config.gen.dart';\n\n")

	// Generate Models
	for _, res := range reg.Resources {
		spec := reg.ResourceSpec(res.Metadata.Name)
		className := toCamelCase(res.Metadata.Name)
		sb.WriteString(fmt.Sprintf("class %s {\n", className))
		sb.WriteString("  final String id;\n")
		for _, f := range spec.Fields {
			if f.Name == "password" || f.Name == "otp_code" || strings.Contains(f.Name, "secret") || strings.Contains(f.Name, "token") {
				continue // Security: Never expose sensitive fields to client models
			}
			dartType := "String"
			if f.Type == "bool" {
				dartType = "bool"
			} else if f.Type == "int" {
				dartType = "int"
			}
			fieldName := toLowerCamelCase(f.Name)
			sb.WriteString(fmt.Sprintf("  final %s? %s;\n", dartType, fieldName))
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s({required this.id, ", className))
		for _, f := range spec.Fields {
			if f.Name == "password" || f.Name == "otp_code" || strings.Contains(f.Name, "secret") || strings.Contains(f.Name, "token") {
				continue
			}
			fieldName := toLowerCamelCase(f.Name)
			sb.WriteString(fmt.Sprintf("this.%s, ", fieldName))
		}
		sb.WriteString("});\n\n")

		sb.WriteString(fmt.Sprintf("  factory %s.fromJson(Map<String, dynamic> json) => %s(\n", className, className))
		sb.WriteString("    id: json['id'] ?? '',\n")
		for _, f := range spec.Fields {
			if f.Name == "password" || f.Name == "otp_code" || strings.Contains(f.Name, "secret") || strings.Contains(f.Name, "token") {
				continue
			}
			fieldName := toLowerCamelCase(f.Name)
			sb.WriteString(fmt.Sprintf("    %s: json['%s'],\n", fieldName, f.Name))
		}
		sb.WriteString("  );\n")
		sb.WriteString("}\n\n")
	}

	// Generate Screen Models
	for _, m := range reg.Screens {
		var spec manifest.ScreenSpec
		m.UnmarshalSpec(&spec)
		className := toCamelCase(m.Metadata.Name)
		sb.WriteString(fmt.Sprintf("class %sScreen {\n", className))
		sb.WriteString("  final String name;\n")
		sb.WriteString("  final String? layout;\n")
		sb.WriteString("  final bool partialSuccess;\n")
		sb.WriteString("  final List<ScreenSection> sections;\n\n")
		sb.WriteString(fmt.Sprintf("  %sScreen({required this.name, this.layout, this.partialSuccess = false, required this.sections});\n", className))
		sb.WriteString("}\n\n")
	}

	sb.WriteString(`class ScreenSection {
  final String key;
  final String? layout;
  final SectionUI? ui;
  final bool defaultVisible;
  ScreenSection({required this.key, this.layout, this.ui, this.defaultVisible = true});
}

class SectionUI {
  final String type;
  final Map<String, dynamic> properties;
  SectionUI({required this.type, this.properties = const {}});
}
`)

	sb.WriteString("class CacheEntry {\n")
	sb.WriteString("  final Map<String, dynamic> data;\n")
	sb.WriteString("  final String? etag;\n")
	sb.WriteString("  CacheEntry(this.data, this.etag);\n")
	sb.WriteString("}\n\n")

	// Generate Client Class
	sb.WriteString("class BFFXClientGenerated {\n")
	sb.WriteString("  String? _token;\n")
	sb.WriteString("  String? _deviceId;\n\n")
	sb.WriteString("  void setAuth(String token) => _token = token;\n")
	sb.WriteString("  void setDeviceId(String id) => _deviceId = id;\n\n")
	if proj.Runtime.Wire.Enabled {
		sb.WriteString("  ClientChannel? _channel;\n")
		sb.WriteString("  ClientChannel get grpcChannel {\n")
		sb.WriteString("    if (_channel == null) {\n")
		sb.WriteString("      _channel = ClientChannel(\n")
		sb.WriteString("        BFFXConfig.grpcHost,\n")
		sb.WriteString("        port: BFFXConfig.grpcPort,\n")
		sb.WriteString("        options: const ChannelOptions(\n")
		sb.WriteString("          credentials: ChannelCredentials.insecure(),\n")
		sb.WriteString("        ),\n")
		sb.WriteString("      );\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return _channel!;\n")
		sb.WriteString("  }\n\n")
		sb.WriteString("  CallOptions get grpcOptions {\n")
		sb.WriteString("    final metadata = <String, String>{};\n")
		sb.WriteString("    if (_token != null) {\n")
		sb.WriteString("      metadata['authorization'] = 'Bearer $_token';\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (_deviceId != null) {\n")
		sb.WriteString("      metadata['x-device-id'] = _deviceId!;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    if (BFFXConfig.appSecret.isNotEmpty) {\n")
		sb.WriteString("      metadata['x-app-secret'] = BFFXConfig.appSecret;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    return CallOptions(metadata: metadata);\n")
		sb.WriteString("  }\n\n")
	}
	sb.WriteString("  final Map<String, CacheEntry> _cache = {};\n\n")

	sb.WriteString("  Future<http.Response> request(String path, {String method = 'GET', Map<String, dynamic>? body, Map<String, String>? queryParameters, String? ifNoneMatch, String? idempotencyKey}) async {\n")
	sb.WriteString("    var uri = Uri.parse('${BFFXConfig.baseUrl}$path');\n")
	sb.WriteString("    if (queryParameters != null && queryParameters.isNotEmpty) {\n")
	sb.WriteString("      uri = uri.replace(queryParameters: queryParameters);\n")
	sb.WriteString("    }\n")
	sb.WriteString("    final headers = <String, String>{\n")
	sb.WriteString("      'Content-Type': 'application/json',\n")
	sb.WriteString("      if (_token != null) 'Authorization': 'Bearer $_token',\n")
	sb.WriteString("      if (_deviceId != null) 'X-Device-ID': _deviceId!,\n")
	sb.WriteString("      if (ifNoneMatch != null) 'If-None-Match': ifNoneMatch,\n")
	sb.WriteString("      if (BFFXConfig.appSecret.isNotEmpty) 'X-App-Secret': BFFXConfig.appSecret,\n")
	sb.WriteString("      'X-BFFX-Client-Version': BFFXConfig.clientVersion,\n")
	sb.WriteString("    };\n")
	sb.WriteString("    try {\n")
	sb.WriteString("      if (Platform.isAndroid) {\n")
	sb.WriteString("        headers['X-BFFX-Platform'] = 'android';\n")
	sb.WriteString("      } else if (Platform.isIOS) {\n")
	sb.WriteString("        headers['X-BFFX-Platform'] = 'ios';\n")
	sb.WriteString("      } else {\n")
	sb.WriteString("        headers['X-BFFX-Platform'] = 'default';\n")
	sb.WriteString("      }\n")
	sb.WriteString("    } catch (_) {\n")
	sb.WriteString("      headers['X-BFFX-Platform'] = 'default';\n")
	sb.WriteString("    }\n")
	sb.WriteString("    final m = method.toUpperCase();\n")
	sb.WriteString("    if (BFFXConfig.idempotencyEnabled && (m == 'POST' || m == 'PUT' || m == 'PATCH' || m == 'DELETE')) {\n")
	sb.WriteString("      headers['X-Idempotency-Key'] = idempotencyKey ?? const Uuid().v4();\n")
	sb.WriteString("    }\n")
	sb.WriteString("    switch (m) {\n")
	sb.WriteString("      case 'POST':\n")
	sb.WriteString("        return http.post(uri, headers: headers, body: jsonEncode(body));\n")
	sb.WriteString("      case 'PUT':\n")
	sb.WriteString("        return http.put(uri, headers: headers, body: jsonEncode(body));\n")
	sb.WriteString("      case 'PATCH':\n")
	sb.WriteString("        return http.patch(uri, headers: headers, body: jsonEncode(body));\n")
	sb.WriteString("      case 'DELETE':\n")
	sb.WriteString("        return http.delete(uri, headers: headers);\n")
	sb.WriteString("      default:\n")
	sb.WriteString("        return http.get(uri, headers: headers);\n")
	sb.WriteString("    }\n")
	sb.WriteString("  }\n\n")

	sb.WriteString(`  Future<Map<String, dynamic>> getWithSWR(String path, {Map<String, String>? queryParameters, Function(Map<String, dynamic>)? onData}) async {
    String? etag;
    final cacheKey = queryParameters == null ? path : '$path?${Uri(queryParameters: queryParameters).query}';
    if (_cache.containsKey(cacheKey)) {
      final entry = _cache[cacheKey]!;
      etag = entry.etag;
      onData?.call(entry.data);
    }
    final res = await request(path, queryParameters: queryParameters, ifNoneMatch: etag);
    if (res.statusCode == 200 || res.statusCode == 206) {
      final data = jsonDecode(res.body);
      final newEtag = res.headers['etag'];
      _cache[cacheKey] = CacheEntry(data, newEtag);
      onData?.call(data);
      return data;
    } else if (res.statusCode == 304) {
      // 304 Not Modified, data in cache is still fresh
      return _cache[cacheKey]!.data;
    }
    throw Exception('fetch failed: ${res.statusCode}');
  }

  /// Stream real-time Server-Sent Events from an endpoint.
  Stream<Map<String, dynamic>> stream(String path, {Map<String, String>? queryParameters}) async* {
    var uri = Uri.parse('${BFFXConfig.baseUrl}$path');
    if (queryParameters != null && queryParameters.isNotEmpty) {
      uri = uri.replace(queryParameters: queryParameters);
    }
    final client = http.Client();
    try {
      final req = http.Request('GET', uri);
      req.headers['Accept'] = 'text/event-stream';
      req.headers['Cache-Control'] = 'no-cache';
      if (_token != null) req.headers['Authorization'] = 'Bearer $_token';
      if (_deviceId != null) req.headers['X-Device-ID'] = _deviceId!;

      final response = await client.send(req);
      String buffer = '';
      await for (final chunk in response.stream.transform(utf8.decoder)) {
        buffer += chunk;
        final lines = buffer.split('\n');
        buffer = lines.removeLast();
        for (final line in lines) {
          final trimmed = line.trim();
          if (trimmed.startsWith('data:')) {
            final payload = trimmed.substring(5).trim();
            if (payload.isNotEmpty && payload != '{}') {
              try {
                yield jsonDecode(payload) as Map<String, dynamic>;
              } catch (_) {}
            }
          }
        }
      }
    } finally {
      client.close();
    }
  }

  /// Subscribe to real-time events on a channel or resource topic.
  Stream<Map<String, dynamic>> subscribe(String channel) {
    return stream('/realtime', queryParameters: {'channel': channel});
  }
`)

	// Screen & Fragment Methods
	for _, m := range reg.Screens {
		var spec manifest.ScreenSpec
		m.UnmarshalSpec(&spec)
		_, path := reg.GetManifestRoute(m)
		path = stripAPIPrefix(path, apiPrefix)
		if path != "" {
			args, dartPath := parsePathParams(path)
			sigArgs := ""
			if len(args) > 0 {
				sigArgs = strings.Join(args, ", ") + ", "
			}
			methodName := toCamelCase(toIdiomaticMethodName(m.Metadata.Name, "screen"))
			sb.WriteString(fmt.Sprintf(`
  Future<Map<String, dynamic>> get%s(%s{Map<String, String>? queryParameters, Function(Map<String, dynamic>)? onData}) {
    return getWithSWR('%s', queryParameters: queryParameters, onData: onData);
  }
`, methodName, sigArgs, dartPath))
		}

		for _, sec := range spec.Sections {
			if sec.Route != nil && sec.Route.Path != "" {
				routePath := stripAPIPrefix(sec.Route.Path, apiPrefix)
				args, dartPath := parsePathParams(routePath)
				sigArgs := ""
				if len(args) > 0 {
					sigArgs = strings.Join(args, ", ") + ", "
				}
				methodName := toCamelCase(toIdiomaticMethodName(m.Metadata.Name+"_"+sec.Key, "section"))
				sb.WriteString(fmt.Sprintf(`
  Future<Map<String, dynamic>> get%s(%s{Map<String, String>? queryParameters, Function(Map<String, dynamic>)? onData}) {
    return getWithSWR('%s', queryParameters: queryParameters, onData: onData);
  }
`, methodName, sigArgs, dartPath))
			}
		}
	}

	// Action Methods
	for _, m := range reg.Actions {
		var spec manifest.ActionSpec
		m.UnmarshalSpec(&spec)
		method, path := reg.GetManifestRoute(m)
		path = stripAPIPrefix(path, apiPrefix)
		if path != "" {
			args, dartPath := parsePathParams(path)
			sigArgs := ""
			if len(args) > 0 {
				sigArgs = strings.Join(args, ", ") + ", "
			}
			methodName := toCamelCase(toIdiomaticMethodName(m.Metadata.Name, "action"))
			prefix := "post"
			if method == "GET" {
				prefix = "get"
			}
			sb.WriteString(fmt.Sprintf(`
  Future<Map<String, dynamic>> %s%s(%sMap<String, dynamic> body, {Map<String, String>? queryParameters}) async {
    final res = await request('%s', method: '%s', body: body, queryParameters: queryParameters);
    if (res.statusCode != 200) {
      throw Exception('action failed: ${res.statusCode} ${res.body}');
    }
    return jsonDecode(res.body);
  }
`, prefix, methodName, sigArgs, dartPath, method))
		}
	}

	if authStrategy == "optional" || authStrategy == "mandatory" {
		sb.WriteString(`
  /// Anonymous JWT session (POST /api/v1/auth/anonymous). Call setDeviceId first or pass [deviceId].
  Future<Map<String, dynamic>> createAnonymousSession({String? deviceId}) async {
    final d = deviceId ?? _deviceId;
    if (d == null || d.isEmpty) {
      throw StateError('Set device id via setDeviceId() or pass deviceId');
    }
    if (deviceId != null) {
      setDeviceId(deviceId);
    }
    final res = await request('/auth/anonymous', method: 'POST', body: {'device_id': d});
    if (res.statusCode != 200) {
      throw Exception('anonymous session failed: ${res.statusCode} ${res.body}');
    }
    final data = jsonDecode(res.body) as Map<String, dynamic>;
    final tok = data['token'] as String?;
    if (tok != null) setAuth(tok);
    return data;
  }

  /// Sign up; pass [anonymousToken] to upgrade the current guest row (same user id).
  Future<http.Response> signUpAccount({
    required String email,
    required String password,
    String? name,
    String? anonymousToken,
  }) {
    return request('/auth/signup', method: 'POST', body: {
      'email': email,
      'password': password,
      if (name != null) 'name': name,
      if (anonymousToken != null) 'anonymous_token': anonymousToken,
    });
  }

  /// Login; pass [anonymousToken] to merge guest-owned rows into this account (server-side).
  Future<Map<String, dynamic>> loginAccount({
    required String email,
    required String password,
    String? anonymousToken,
  }) async {
    final res = await request('/auth/login', method: 'POST', body: {
      'email': email,
      'password': password,
      if (anonymousToken != null) 'anonymous_token': anonymousToken,
    });
    if (res.statusCode != 200) {
      throw Exception('login failed: ${res.statusCode} ${res.body}');
    }
    final data = jsonDecode(res.body) as Map<String, dynamic>;
    final tok = data['token'] as String?;
    if (tok != null) setAuth(tok);
    return data;
  }

  /// Logout; revokes the current JTI on the server (if Redis is enabled) and clears local token.
  Future<http.Response> logout() async {
    final res = await request('/auth/logout', method: 'POST');
    if (res.statusCode == 200) {
      _token = null;
    }
    return res;
  }
`)
	}
	sb.WriteString("}\n\n")

	sb.WriteString(`class FeatureFlagService {
  final Map<String, dynamic> _flags;
  final Map<String, Map<String, dynamic>> _screens;

  FeatureFlagService(Map<String, dynamic> bootstrapData)
      : _flags = bootstrapData['flags'] ?? {},
        _screens = (bootstrapData['screens'] as Map<String, dynamic>? ?? {}).map(
          (k, v) => MapEntry(k, Map<String, dynamic>.from(v as Map)),
        );

  bool isEnabled(String key) {
    final flag = _flags[key];
    if (flag == null) return false;
    return flag['value'] == true;
  }

  dynamic getVariation(String key) {
    final flag = _flags[key];
    return flag != null ? flag['value'] : null;
  }

  bool isSectionVisible(String screen, String section) {
    final screenData = _screens[screen];
    if (screenData == null) return true;
    final sectionData = screenData[section];
    if (sectionData == null) return true;
    return sectionData['visible'] == true;
  }
}

// Simple FeatureGate widget (requires Provider or similar)
/*
class FeatureGate extends StatelessWidget {
  final String? flag;
  final String? screen;
  final String? section;
  final Widget child;
  final Widget fallback;

  const FeatureGate({
    super.key,
    this.flag,
    this.screen,
    this.section,
    required this.child,
    this.fallback = const SizedBox.shrink(),
  });

  @override
  Widget build(BuildContext context) {
    final service = Provider.of<FeatureFlagService>(context);
    bool visible = true;
    if (flag != null) {
      visible = service.isEnabled(flag!);
    } else if (screen != null && section != null) {
      visible = service.isSectionVisible(screen!, section!);
    }
    return visible ? child : fallback;
  }
}
*/
`)
	if err := os.WriteFile(filepath.Join(clientDir, "bffx_client.gen.dart"), []byte(sb.String()), 0o644); err != nil {
		return err
	}

	// 3. Ensure manual entry point exists (do not overwrite if present)
	entryPoint := filepath.Join(clientDir, "bffx_client.dart")
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		entryContent := `import 'bffx_client.gen.dart';

// Use this file for manual overrides or extensions to the generated GeneratedBFFXClient.
class BFFXClient extends BFFXClientGenerated {
  // Add your custom methods or overrides here
}
`
		if err := os.WriteFile(entryPoint, []byte(entryContent), 0o644); err != nil {
			return err
		}
	}

	// 4. Generate bffx_grpc_client.gen.dart when gRPC is enabled on the
	// project AND at least one screen has gRPC transport. The file is a
	// thin facade over the protoc-generated stubs (which live under
	// `lib/gen/`); it is safe to emit even if protoc has not yet been run
	// because the imports will simply be unused.
	if proj.Runtime.Wire.Enabled && hasGRPCScreens(reg) {
		grpcPath := filepath.Join(clientDir, "bffx_grpc_client.gen.dart")
		if err := os.WriteFile(grpcPath, []byte(buildDartScreenGRPCFacade(reg)), 0o644); err != nil {
			return err
		}

		// Automate Dart/gRPC stub generation
		genDir := filepath.Join(clientDir, "gen")
		if err := os.MkdirAll(genDir, 0o755); err != nil {
			return fmt.Errorf("create gen dir: %w", err)
		}

		protoDir := filepath.Join(root, ".bffx", "proto")
		protoFile := filepath.Join(protoDir, "bffx", "v1", "service.proto")

		pathEnv := os.Getenv("PATH")
		homeDir, _ := os.UserHomeDir()
		goBin := filepath.Join(homeDir, "go", "bin")
		pubBin := filepath.Join(homeDir, ".pub-cache", "bin")
		brewBin := "/opt/homebrew/bin"
		usrLocalBin := "/usr/local/bin"
		newPath := fmt.Sprintf("%s:%s:%s:%s:%s", goBin, pubBin, brewBin, usrLocalBin, pathEnv)

		cmd := exec.Command("protoc",
			"--dart_out=grpc:"+genDir,
			"-I"+protoDir,
			protoFile,
		)
		cmd.Env = append(os.Environ(), "PATH="+newPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Warning: Failed to auto-generate Dart gRPC stubs using protoc: %v\nOutput: %s\nMake sure protoc and protoc-gen-dart are installed in your PATH.\n", err, string(output))
		} else {
			fmt.Println("Successfully generated Dart gRPC stubs using protoc")
		}
	}

	return nil
}

func hasGRPCScreens(reg *manifest.Registry) bool {
	for _, s := range reg.Screens {
		var spec manifest.ScreenSpec
		if err := s.UnmarshalSpec(&spec); err != nil {
			continue
		}
		if len(spec.Transports) == 0 {
			return true
		}
		for _, t := range spec.Transports {
			if strings.EqualFold(t, "grpc") {
				return true
			}
		}
	}
	return false
}

// buildDartScreenGRPCFacade emits a Dart file that exposes a strongly-typed
// gRPC client for each screen. It assumes that `protoc --dart_out=lib/gen
// --grpc_dart_out=lib/gen` has been run against the proto file produced by
// `bffx sync`; the facade simply wires those stubs to BFFXConfig + auth
// metadata so that mobile code can write `client.userProfile.get()` instead
// of dealing with channels directly.
func buildDartScreenGRPCFacade(reg *manifest.Registry) string {
	var sb strings.Builder
	sb.WriteString("// GENERATED CODE - DO NOT MODIFY BY HAND\n")
	sb.WriteString("// Strongly typed gRPC facade for App Features screens.\n")
	sb.WriteString("// Requires `protoc --dart_out=lib/gen --grpc_dart_out=lib/gen` to have been\n")
	sb.WriteString("// executed against the bffx-generated .proto file.\n\n")
	sb.WriteString("import 'package:grpc/grpc.dart';\n")
	sb.WriteString("import 'gen/bffx/v1/service.pbgrpc.dart';\n")
	sb.WriteString("import 'bffx_config.gen.dart';\n\n")

	sb.WriteString("class BFFXScreenGRPCClient {\n")
	sb.WriteString("  final String? bearerToken;\n")
	sb.WriteString("  final String? deviceId;\n")
	sb.WriteString("  late final ClientChannel _channel;\n\n")

	// Screen stub fields
	for _, s := range reg.Screens {
		var spec manifest.ScreenSpec
		_ = s.UnmarshalSpec(&spec)
		if len(spec.Transports) > 0 && !containsCaseInsensitive(spec.Transports, "grpc") {
			continue
		}
		title := dartScreenTitle(s, &spec)
		field := toLowerCamelCase(dartScreenSourceName(s, &spec))
		sb.WriteString(fmt.Sprintf("  late final %sScreenServiceClient %s;\n", title, field))
	}
	sb.WriteString("\n")

	sb.WriteString("  BFFXScreenGRPCClient({this.bearerToken, this.deviceId}) {\n")
	sb.WriteString("    _channel = ClientChannel(\n")
	sb.WriteString("      BFFXConfig.grpcHost,\n")
	sb.WriteString("      port: BFFXConfig.grpcPort,\n")
	sb.WriteString("      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),\n")
	sb.WriteString("    );\n")
	for _, s := range reg.Screens {
		var spec manifest.ScreenSpec
		_ = s.UnmarshalSpec(&spec)
		if len(spec.Transports) > 0 && !containsCaseInsensitive(spec.Transports, "grpc") {
			continue
		}
		title := dartScreenTitle(s, &spec)
		field := toLowerCamelCase(dartScreenSourceName(s, &spec))
		sb.WriteString(fmt.Sprintf("    %s = %sScreenServiceClient(_channel, options: _callOptions());\n", field, title))
	}
	sb.WriteString("  }\n\n")

	sb.WriteString("  CallOptions _callOptions() {\n")
	sb.WriteString("    final metadata = <String, String>{};\n")
	sb.WriteString("    if (bearerToken != null) metadata['authorization'] = 'Bearer ' + bearerToken!;\n")
	sb.WriteString("    if (deviceId != null) metadata['x-device-id'] = deviceId!;\n")
	sb.WriteString("    if (BFFXConfig.appSecret.isNotEmpty) metadata['x-app-secret'] = BFFXConfig.appSecret;\n")
	sb.WriteString("    return CallOptions(metadata: metadata);\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  Future<void> close() => _channel.shutdown();\n")
	sb.WriteString("}\n")
	return sb.String()
}

// dartScreenSourceName returns the snake_case identifier used as the source
// of TitleCase / camelCase derivations. It mirrors `screenProtoTitle` in
// `pkg/batteries/wire/proto/builder.go` so the Dart facade references the
// exact symbol the proto generator emitted.
func dartScreenSourceName(m *manifest.Manifest, spec *manifest.ScreenSpec) string {
	if spec != nil && strings.TrimSpace(spec.Name) != "" {
		return spec.Name
	}
	name := m.Metadata.Name
	if strings.HasSuffix(name, "_screen") {
		name = strings.TrimSuffix(name, "_screen")
	}
	return name
}

func dartScreenTitle(m *manifest.Manifest, spec *manifest.ScreenSpec) string {
	return toCamelCase(dartScreenSourceName(m, spec))
}

func containsCaseInsensitive(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

func toIdiomaticMethodName(name string, kind string) string {
	s := name
	s = strings.TrimPrefix(s, "features_")
	s = strings.TrimSuffix(s, "_action")
	s = strings.TrimSuffix(s, "_screen")
	s = strings.TrimSuffix(s, "_builder")

	return s
}

func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func toLowerCamelCase(s string) string {
	res := toCamelCase(s)
	if len(res) > 0 {
		return strings.ToLower(res[:1]) + res[1:]
	}
	return res
}

func stripAPIPrefix(path, apiPrefix string) string {
	if path == "" || apiPrefix == "" {
		return path
	}
	if path == apiPrefix {
		return "/"
	}
	if strings.HasPrefix(path, apiPrefix+"/") {
		return path[len(apiPrefix):]
	}
	return path
}
