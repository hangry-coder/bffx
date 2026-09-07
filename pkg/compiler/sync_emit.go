package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func emitGoArtifacts(root string, actions []string, reg *manifest.Registry) error {
	modPath := getModulePath(root)
	if modPath == "" {
		return fmt.Errorf("go.mod not found or invalid in %s", root)
	}

	actions = append([]string(nil), actions...)
	sort.Strings(actions)

	hooksDir := filepath.Join(root, "hooks")

	cmdDirName := "orchestrator"
	if _, err := os.Stat(filepath.Join(root, "cmd", "api")); err == nil {
		cmdDirName = "api"
	}
	cmdDir := filepath.Join(root, "cmd", cmdDirName)
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return fmt.Errorf("make cmd dir: %w", err)
	}

	// Emit registry.gen.go
	var actionEntries []string
	var hookEntries []string

	// Discover hooks based on layout type
	var isV2 bool
	var projectSpec manifest.ProjectSpec
	if reg.Project != nil {
		_ = reg.Project.UnmarshalSpec(&projectSpec)
	}
	if projectSpec.Layout == "v2" {
		isV2 = true
	} else {
		// Legacy layout keeps a top-level hooks/ package for hand-written handlers.
		_ = os.MkdirAll(hooksDir, 0o755)
	}

	hookScan := scanHookPackages(root, isV2)
	lifecycleHookFuncs := hookScan.Lifecycle
	payloadHookFuncs := hookScan.Payload
	hookFuncToFeature := hookScan.FuncFeature

	usedFeatures := make(map[string]bool)

	// 2. Register Actions
	for _, a := range actions {
		var actionFeature string
		for _, act := range reg.Actions {
			if act.Metadata.Name == a || strings.ToLower(act.Metadata.Name) == strings.ToLower(a) || strings.ReplaceAll(strings.ToLower(act.Metadata.Name), "_action", "") == strings.ReplaceAll(strings.ToLower(a), "_action", "") {
				actionFeature = getFeatureOfManifest(act)
				break
			}
		}
		if actionFeature == "" {
			actionFeature = "app"
		}

		hookName := a
		hookName = strings.TrimPrefix(hookName, "features_")
		hookName = strings.TrimSuffix(hookName, "_action")

		parts := strings.Split(hookName, "_")
		if len(parts) > 0 {
			parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
		}
		hookName = strings.Join(parts, "_")

		var prefix string
		if isV2 {
			scannedFeature, found := hookFuncToFeature["Handle"+hookName]
			if found {
				actionFeature = scannedFeature
			}
			prefix = actionFeature + "hooks."
			usedFeatures[actionFeature] = true
		} else {
			prefix = "hooks."
		}

		actionEntries = append(actionEntries, fmt.Sprintf("\t\"%s\": %sHandle%s,", a, prefix, hookName))
	}

	registeredHookKeys := make(map[string]bool)

	// 3. Register lifecycle hooks (Resource Hooks)
	resList := append([]*manifest.Manifest(nil), reg.Resources...)
	sort.Slice(resList, func(i, j int) bool {
		return resList[i].Metadata.Name < resList[j].Metadata.Name
	})
	for _, m := range resList {
		r := m.Metadata.Name
		resourceFeature := getFeatureOfManifest(m)
		lifecycle := []string{"BeforeCreate", "AfterCreate", "BeforeUpdate", "AfterUpdate", "BeforeDelete", "AfterDelete"}
		for _, l := range lifecycle {
			hookName := l + r
			if lifecycleHookFuncs[hookName] {
				var prefix string
				if isV2 {
					scannedFeature, found := hookFuncToFeature[hookName]
					if found {
						resourceFeature = scannedFeature
					}
					prefix = resourceFeature + "hooks."
					usedFeatures[resourceFeature] = true
				} else {
					prefix = "hooks."
				}
				hookEntries = append(hookEntries, fmt.Sprintf("\t\"%s\": %s%s,", hookName, prefix, hookName))
				registeredHookKeys[hookName] = true
			}
		}
	}

	// 4. Register payload-style hooks (SendOTP, etc.)
	var payloadNames []string
	for n := range payloadHookFuncs {
		payloadNames = append(payloadNames, n)
	}
	sort.Strings(payloadNames)
	for _, n := range payloadNames {
		if registeredHookKeys[n] {
			continue
		}
		var prefix string
		if isV2 {
			payloadFeature := "app"
			scannedFeature, found := hookFuncToFeature[n]
			if found {
				payloadFeature = scannedFeature
			}
			prefix = payloadFeature + "hooks."
			usedFeatures[payloadFeature] = true
		} else {
			prefix = "hooks."
		}
		hookEntries = append(hookEntries, fmt.Sprintf("\t\"%s\": %s%s,", n, prefix, n))
		registeredHookKeys[n] = true
	}

	warnUnresolvedManifestHooks(registeredHookKeys, collectManifestHookActions(reg))

	importSection := []string{
		"\t\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
		"\t\"github.com/hangry-coder/bffx/pkg/api/router\"",
	}
	if isV2 {
		var sortedFeatures []string
		for f := range usedFeatures {
			sortedFeatures = append(sortedFeatures, f)
		}
		sort.Strings(sortedFeatures)
		for _, f := range sortedFeatures {
			importSection = append(importSection, fmt.Sprintf("\t%shooks \"%s/internal/features/%s/hooks\"", f, modPath, f))
		}
	} else {
		if len(actionEntries) > 0 || len(hookEntries) > 0 {
			importSection = append(importSection, fmt.Sprintf("\t\"%s/hooks\"", modPath))
		}
	}

	var grpcInit string
	var grpcImplementations string

	if projectSpec.Runtime.Wire.Enabled {
		importSection = append(importSection,
			fmt.Sprintf("\tpb \"%s/.bffx/gen/go/proto/bffx/v1\"", modPath),
			"\t\"google.golang.org/protobuf/encoding/protojson\"",
			"\t\"google.golang.org/protobuf/types/known/structpb\"",
			"\t\"google.golang.org/grpc\"",
			"\t\"bytes\"",
			"\t\"io\"",
			"\t\"net/http\"",
			"\t\"net/http/httptest\"",
			"\t\"google.golang.org/grpc/metadata\"",
			"\t\"google.golang.org/grpc/status\"",
			"\t\"google.golang.org/grpc/codes\"",
			"\t\"github.com/hangry-coder/bffx/pkg/app\"",
			"\t\"context\"",
			"\t\"encoding/json\"",
		)

		var grpcRegistrations []string
		var grpcServers []string

		for _, m := range reg.Resources {
			var spec manifest.ResourceSpec
			_ = m.UnmarshalSpec(&spec)
			if len(spec.Transports) > 0 {
				grpcAllowed := false
				for _, t := range spec.Transports {
					if strings.ToLower(t) == "grpc" {
						grpcAllowed = true
						break
					}
				}
				if !grpcAllowed {
					continue
				}
			}
			if !spec.Routes.Crud {
				continue
			}

			R := m.Metadata.Name
			pluralR := R + "s"

			serviceServer := toCamelCase(R) + "ServiceServer"
			resourceMsg := toCamelCase(R)
			listMethod := toGRPCName("List", pluralR)
			listReq := listMethod + "Request"
			listResp := listMethod + "Response"
			getMethod := toGRPCName("Get", R)
			getReq := getMethod + "Request"
			createMethod := toGRPCName("Create", R)
			createReq := createMethod + "Request"
			updateMethod := toGRPCName("Update", R)
			updateReq := updateMethod + "Request"
			deleteMethod := toGRPCName("Delete", R)
			deleteReq := deleteMethod + "Request"
			deleteResp := deleteMethod + "Response"

			grpcRegistrations = append(grpcRegistrations, fmt.Sprintf("\tpb.Register%s(s, &%s{router: r})", serviceServer, serviceServer))

			grpcServers = append(grpcServers, fmt.Sprintf(`
type %s struct {
	pb.Unimplemented%s
	router *router.Router
}

func (s *%s) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	limit := int(req.Limit)
	if limit == 0 {
		limit = 100
	}
	items, err := s.router.Store().List(ctx, "%s", limit, int(req.Offset))
	if err != nil {
		return nil, err
	}
	var pbItems []*pb.%s
	for _, item := range items {
		jsonBytes, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		pbItem := &pb.%s{}
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(jsonBytes, pbItem); err != nil {
			return nil, err
		}
		pbItems = append(pbItems, pbItem)
	}
	return &pb.%s{
		Items: pbItems,
		Total: int32(len(items)),
	}, nil
}

func (s *%s) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	item, err := s.router.Store().Get(ctx, "%s", req.Id)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	pbItem := &pb.%s{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(jsonBytes, pbItem); err != nil {
		return nil, err
	}
	return pbItem, nil
}

func (s *%s) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	payload, err := s.router.MarshalProtoResourcePayload("%s", req)
	if err != nil {
		return nil, err
	}
	item, err := s.router.CreateResourceGRPC(ctx, "%s", payload)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	pbItem := &pb.%s{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(jsonBytes, pbItem); err != nil {
		return nil, err
	}
	return pbItem, nil
}

func (s *%s) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	payload, err := s.router.MarshalProtoResourcePayload("%s", req)
	if err != nil {
		return nil, err
	}
	item, err := s.router.UpdateResourceGRPC(ctx, "%s", req.Id, payload)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	pbItem := &pb.%s{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(jsonBytes, pbItem); err != nil {
		return nil, err
	}
	return pbItem, nil
}

func (s *%s) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	err := s.router.Store().Delete(ctx, "%s", req.Id)
	if err != nil {
		return &pb.%s{Success: false}, err
	}
	return &pb.%s{Success: true}, nil
}
`,
				serviceServer, serviceServer,
				serviceServer, listMethod, listReq, listResp, R, resourceMsg, resourceMsg, listResp,
				serviceServer, getMethod, getReq, resourceMsg, R, resourceMsg,
				serviceServer, createMethod, createReq, resourceMsg, R, R, resourceMsg,
				serviceServer, updateMethod, updateReq, resourceMsg, R, R, resourceMsg,
				serviceServer, deleteMethod, deleteReq, deleteResp, R, deleteResp, deleteResp,
			))
		}

		var actionMethods []string
		for _, a := range reg.Actions {
			var spec manifest.ActionSpec
			_ = a.UnmarshalSpec(&spec)
			if len(spec.Transports) > 0 {
				grpcAllowed := false
				for _, t := range spec.Transports {
					if strings.ToLower(t) == "grpc" {
						grpcAllowed = true
						break
					}
				}
				if !grpcAllowed {
					continue
				}
			}

			methodName := toCamelCase(a.Metadata.Name)
			reqType := methodName + "Request"
			respType := methodName + "Response"

			actionMethods = append(actionMethods, fmt.Sprintf(`
func (s *ActionServiceServer) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	var payloadBytes []byte
	if req.Payload != nil {
		jsonBytes, err := protojson.Marshal(req.Payload)
		if err != nil {
			return nil, err
		}
		payloadBytes = jsonBytes
	}

	method, path := "POST", ""
	for _, act := range s.router.Registry().Actions {
		if act.Metadata.Name == "%s" {
			method, path = s.router.Registry().GetManifestRoute(act)
			break
		}
	}
	if path == "" {
		path = s.router.Registry().ApiPrefix + "/actions/%s"
	}

	httpReq, err := router.NewGRPCProxyHTTPRequest(ctx, method, path, payloadBytes)
	if err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()
	s.router.Handler().ServeHTTP(rec, httpReq)

	if rec.Code >= 400 {
		return nil, status.Errorf(codes.Code(rec.Code), "action failed: %%d %%s", rec.Code, rec.Body.String())
	}

	var pbResult structpb.Struct
	if rec.Body.Len() > 0 {
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(rec.Body.Bytes(), &pbResult); err != nil {
			return nil, err
		}
	}

	return &pb.%s{
		Result: &pbResult,
	}, nil
}
`, methodName, reqType, respType, a.Metadata.Name, a.Metadata.Name, respType))
		}

		if len(actionMethods) > 0 {
			grpcRegistrations = append(grpcRegistrations, "\tpb.RegisterActionServiceServer(s, &ActionServiceServer{router: r})")
			grpcServers = append(grpcServers, fmt.Sprintf(`
type ActionServiceServer struct {
	pb.UnimplementedActionServiceServer
	router *router.Router
}
%s
`, strings.Join(actionMethods, "\n")))
		}

		var builderMethods []string
		for _, b := range reg.Builders {
			var spec manifest.BuilderSpec
			_ = b.UnmarshalSpec(&spec)
			if len(spec.Transports) > 0 {
				grpcAllowed := false
				for _, t := range spec.Transports {
					if strings.ToLower(t) == "grpc" {
						grpcAllowed = true
						break
					}
				}
				if !grpcAllowed {
					continue
				}
			}

			methodName := toGRPCName("Get", b.Metadata.Name)
			reqType := methodName + "Request"
			respType := methodName + "Response"

			builderMethods = append(builderMethods, fmt.Sprintf(`
func (s *BuilderServiceServer) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	var payloadBytes []byte
	if req.Params != nil {
		jsonBytes, err := protojson.Marshal(req.Params)
		if err != nil {
			return nil, err
		}
		payloadBytes = jsonBytes
	}

	method, path := "GET", ""
	for _, bld := range s.router.Registry().Builders {
		if bld.Metadata.Name == "%s" {
			method, path = s.router.Registry().GetManifestRoute(bld)
			break
		}
	}
	if path == "" {
		path = s.router.Registry().ApiPrefix + "/builders/%s"
	}

	httpReq, err := router.NewGRPCProxyHTTPRequest(ctx, method, path, payloadBytes)
	if err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()
	s.router.Handler().ServeHTTP(rec, httpReq)

	if rec.Code >= 400 {
		return nil, status.Errorf(codes.Code(rec.Code), "builder failed: %%d %%s", rec.Code, rec.Body.String())
	}

	var pbResult structpb.Struct
	if rec.Body.Len() > 0 {
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(rec.Body.Bytes(), &pbResult); err != nil {
			return nil, err
		}
	}

	return &pb.%s{
		Result: &pbResult,
	}, nil
}
`, methodName, reqType, respType, b.Metadata.Name, b.Metadata.Name, respType))
		}

		if len(builderMethods) > 0 {
			grpcRegistrations = append(grpcRegistrations, "\tpb.RegisterBuilderServiceServer(s, &BuilderServiceServer{router: r})")
			grpcServers = append(grpcServers, fmt.Sprintf(`
type BuilderServiceServer struct {
	pb.UnimplementedBuilderServiceServer
	router *router.Router
}
%s
`, strings.Join(builderMethods, "\n")))
		}

		var pipelineMethods []string
		for _, p := range reg.Pipelines {
			var spec manifest.PipelineSpec
			_ = p.UnmarshalSpec(&spec)
			if len(spec.Transports) > 0 {
				grpcAllowed := false
				for _, t := range spec.Transports {
					if strings.ToLower(t) == "grpc" {
						grpcAllowed = true
						break
					}
				}
				if !grpcAllowed {
					continue
				}
			}

			methodName := toGRPCName("Process", p.Metadata.Name)
			reqType := methodName + "Request"
			respType := methodName + "Response"

			pipelineMethods = append(pipelineMethods, fmt.Sprintf(`
func (s *PipelineServiceServer) %s(ctx context.Context, req *pb.%s) (*pb.%s, error) {
	var payloadBytes []byte
	if req.Payload != nil {
		jsonBytes, err := protojson.Marshal(req.Payload)
		if err != nil {
			return nil, err
		}
		payloadBytes = jsonBytes
	}

	method, path := "POST", ""
	for _, pipe := range s.router.Registry().Pipelines {
		if pipe.Metadata.Name == "%s" {
			method, path = s.router.Registry().GetManifestRoute(pipe)
			break
		}
	}
	if path == "" {
		path = s.router.Registry().ApiPrefix + "/pipelines/%s"
	}

	httpReq, err := router.NewGRPCProxyHTTPRequest(ctx, method, path, payloadBytes)
	if err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()
	s.router.Handler().ServeHTTP(rec, httpReq)

	if rec.Code >= 400 {
		return nil, status.Errorf(codes.Code(rec.Code), "pipeline failed: %%d %%s", rec.Code, rec.Body.String())
	}

	var pbResult structpb.Struct
	if rec.Body.Len() > 0 {
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(rec.Body.Bytes(), &pbResult); err != nil {
			return nil, err
		}
	}

	return &pb.%s{
		Result: &pbResult,
	}, nil
}
`, methodName, reqType, respType, p.Metadata.Name, p.Metadata.Name, respType))
		}

		if len(pipelineMethods) > 0 {
			grpcRegistrations = append(grpcRegistrations, "\tpb.RegisterPipelineServiceServer(s, &PipelineServiceServer{router: r})")
			grpcServers = append(grpcServers, fmt.Sprintf(`
type PipelineServiceServer struct {
	pb.UnimplementedPipelineServiceServer
	router *router.Router
}
%s
`, strings.Join(pipelineMethods, "\n")))
		}

		grpcInit = fmt.Sprintf(`
func init() {
	app.GRPCRegister = func(s *grpc.Server, r *router.Router) {
%s
	}
}
`, strings.Join(grpcRegistrations, "\n"))

		dummyUsage := `
var _ = bytes.MinRead
var _ = io.EOF
var _ = http.MethodGet
var _ = httptest.NewRecorder
var _ = metadata.Pairs
var _ = status.Error
var _ = codes.OK
var _ = protojson.Marshal
var _ = structpb.NewStruct
`
		grpcImplementations = dummyUsage + "\n" + strings.Join(grpcServers, "\n")
	}

	registryCode := strings.Join([]string{
		"package main",
		"",
		"import (",
		strings.Join(importSection, "\n"),
		")",
		"",
		"// Code generated by bffx sync. DO NOT EDIT.",
		"var ActionHandlers = map[string]handlers.ActionHandler{",
		strings.Join(actionEntries, "\n"),
		"}",
		"",
		"var HookHandlers = map[string]router.HookFunc{",
		strings.Join(hookEntries, "\n"),
		"}",
		"",
		grpcInit,
		grpcImplementations,
	}, "\n")

	if err := os.WriteFile(filepath.Join(cmdDir, "registry.gen.go"), []byte(registryCode), 0o644); err != nil {
		return fmt.Errorf("write registry.gen.go: %w", err)
	}

	// Keep cmd/api/main.go in sync with registry.gen.go (wires ActionHandlers + HookHandlers).
	if err := generator.GenerateOrchestratorMain(root, false); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	return nil
}
