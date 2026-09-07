package cli

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

type RouteEntry struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Auth   string `json:"auth,omitempty"`
}

func HandleRoutes(args []string) {
	root := "."
	formatJSON := false
	filter := ""

	for i := 0; i < len(args); i++ {
		if args[i] == "list" {
			continue
		}
		if args[i] == "help" || args[i] == "-h" || args[i] == "--help" {
			fmt.Println("Usage: bffx routes [list] [--root DIR] [--json] [--filter KIND]")
			return
		}
		switch args[i] {
		case "--root":
			if i+1 < len(args) {
				root = args[i+1]
				i++
			}
		case "--json":
			formatJSON = true
		case "--filter":
			if i+1 < len(args) {
				filter = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "--") && filter == "" {
				filter = args[i]
			}
		}
	}

	reg, err := manifest.LoadAll(root)
	if err != nil {
		log.Fatalf("failed to load manifests: %v", err)
	}

	routes := collectRoutes(reg)

	if filter != "" {
		filtered := []RouteEntry{}
		f := strings.ToLower(filter)
		for _, r := range routes {
			// Check kind match first
			if strings.EqualFold(r.Kind, f) {
				filtered = append(filtered, r)
				continue
			}
			// General contains search
			if strings.Contains(strings.ToLower(r.Kind), f) ||
				strings.Contains(strings.ToLower(r.Path), f) ||
				strings.Contains(strings.ToLower(r.Name), f) {
				filtered = append(filtered, r)
			}
		}
		routes = filtered
	}

	if formatJSON {
		printRoutesJSON(routes)
	} else {
		printRoutesTable(routes)
	}
}

func collectRoutes(reg *manifest.Registry) []RouteEntry {
	var routes []RouteEntry
	// 1. Built-in System Routes
	routes = append(routes, RouteEntry{Method: "GET", Path: "/health", Kind: "Builtin", Name: "Health"})
	routes = append(routes, RouteEntry{Method: "GET", Path: "/metrics", Kind: "Builtin", Name: "Metrics", Auth: "internal"})
	routes = append(routes, RouteEntry{Method: "GET", Path: reg.ApiPrefix + "/hello", Kind: "Builtin", Name: "Welcome"})
	routes = append(routes, RouteEntry{Method: "GET", Path: reg.ApiPrefix + "/app/bootstrap", Kind: "Builtin", Name: "Bootstrap"})
	routes = append(routes, RouteEntry{Method: "GET", Path: reg.ApiPrefix + "/ui/registry", Kind: "Builtin", Name: "UIRegistry"})

	// 2. Auth Routes
	var projectSpec manifest.ProjectSpec
	reg.Project.UnmarshalSpec(&projectSpec)
	if projectSpec.App.AuthStrategy != "" {
		routes = append(routes, RouteEntry{Method: "POST", Path: reg.ApiPrefix + "/auth/signup", Kind: "Auth", Name: "Signup", Auth: "public"})
		routes = append(routes, RouteEntry{Method: "POST", Path: reg.ApiPrefix + "/auth/login", Kind: "Auth", Name: "Login", Auth: "public"})
		routes = append(routes, RouteEntry{Method: "POST", Path: reg.ApiPrefix + "/auth/refresh", Kind: "Auth", Name: "Refresh", Auth: "public"})
		routes = append(routes, RouteEntry{Method: "GET", Path: reg.ApiPrefix + "/auth/handshake", Kind: "Auth", Name: "Handshake", Auth: "public"})
		routes = append(routes, RouteEntry{Method: "POST", Path: reg.ApiPrefix + "/auth/logout", Kind: "Auth", Name: "Logout", Auth: "required"})
		routes = append(routes, RouteEntry{Method: "POST", Path: reg.ApiPrefix + "/auth/anonymous", Kind: "Auth", Name: "Anonymous", Auth: "public"})
		routes = append(routes, RouteEntry{Method: "GET", Path: reg.ApiPrefix + "/me", Kind: "Auth", Name: "CurrentUser", Auth: "required"})
	}

	// 3. API Docs
	routes = append(routes, RouteEntry{Method: "GET", Path: "/api/admin/docs", Kind: "Docs", Name: "SwaggerUI", Auth: "admin"})
	routes = append(routes, RouteEntry{Method: "GET", Path: "/api/admin/docs/openapi.json", Kind: "Docs", Name: "OpenAPISpec", Auth: "admin"})

	// 4. Resource CRUD
	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		r.UnmarshalSpec(&spec)
		if spec.Routes.Crud {
			name := strings.ToLower(r.Metadata.Name)
			collPath := spec.Routes.CollectionPath
			if collPath == "" {
				collPath = name + "s"
			}
			authInfo := fmt.Sprintf("R:%v,W:%v", spec.Policy.Read, spec.Policy.Write)

			routes = append(routes, RouteEntry{
				Method: "GET/POST",
				Path:   reg.ApiPrefix + "/" + collPath,
				Kind:   "Resource",
				Name:   r.Metadata.Name,
				Auth:   authInfo,
			})
			routes = append(routes, RouteEntry{
				Method: "GET/PATCH/DELETE",
				Path:   reg.ApiPrefix + "/" + collPath + "/:id",
				Kind:   "Resource",
				Name:   r.Metadata.Name,
				Auth:   authInfo,
			})
		}
	}

	// 5. Screens
	for _, s := range reg.Screens {
		method, path := reg.GetManifestRoute(s)
		var spec manifest.ScreenSpec
		s.UnmarshalSpec(&spec)
		routes = append(routes, RouteEntry{
			Method: method,
			Path:   path,
			Kind:   "Screen",
			Name:   s.Metadata.Name,
			Auth:   spec.Route.Auth,
		})
	}

	// 6. Actions
	for _, a := range reg.Actions {
		method, path := reg.GetManifestRoute(a)
		var spec manifest.ActionSpec
		a.UnmarshalSpec(&spec)
		routes = append(routes, RouteEntry{
			Method: method,
			Path:   path,
			Kind:   "Action",
			Name:   a.Metadata.Name,
			Auth:   spec.Route.Auth,
		})
	}

	// 7. Builders
	for _, b := range reg.Builders {
		method, path := reg.GetManifestRoute(b)
		var spec manifest.BuilderSpec
		b.UnmarshalSpec(&spec)
		routes = append(routes, RouteEntry{
			Method: method,
			Path:   path,
			Kind:   "Builder",
			Name:   b.Metadata.Name,
			Auth:   spec.Route.Auth,
		})
	}

	// 8. Streams
	for _, s := range reg.Streams {
		var spec manifest.StreamSpec
		s.UnmarshalSpec(&spec)
		routes = append(routes, RouteEntry{
			Method: "SSE",
			Path:   spec.Route.Path,
			Kind:   "Stream",
			Name:   s.Metadata.Name,
			Auth:   spec.Route.Auth,
		})
	}

	// 9. CronJobs (declarative; executed by worker scheduler)
	for _, c := range reg.CronJobs {
		var spec manifest.CronJobSpec
		c.UnmarshalSpec(&spec)
		routes = append(routes, RouteEntry{
			Method: "CRON",
			Path:   spec.Schedule,
			Kind:   "CronJob",
			Name:   c.Metadata.Name + " → " + spec.Action,
			Auth:   "worker",
		})
	}

	// 10. Jobs
	routes = append(routes, RouteEntry{Method: "POST", Path: reg.ApiPrefix + "/jobs/dispatch", Kind: "Jobs", Name: "Dispatch", Auth: "required"})
	routes = append(routes, RouteEntry{Method: "GET", Path: reg.ApiPrefix + "/jobs", Kind: "Jobs", Name: "List", Auth: "required"})
	routes = append(routes, RouteEntry{Method: "PATCH", Path: reg.ApiPrefix + "/jobs/:id/result", Kind: "Jobs", Name: "WriteBack", Auth: "worker"})

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Kind != routes[j].Kind {
			return routes[i].Kind < routes[j].Kind
		}
		return routes[i].Path < routes[j].Path
	})

	return routes
}

func printRoutesTable(routes []RouteEntry) {
	fmt.Printf("%-18s %-40s %-30s %s\n", "METHOD", "PATH", "NAME", "AUTH")
	fmt.Println(strings.Repeat("-", 100))

	currentKind := ""
	for _, r := range routes {
		if r.Kind != currentKind {
			if currentKind != "" {
				fmt.Println()
			}
			fmt.Printf("[%s]\n", strings.ToUpper(r.Kind))
			currentKind = r.Kind
		}
		authStr := ""
		if r.Auth != "" {
			authStr = fmt.Sprintf("[%s]", r.Auth)
		}
		fmt.Printf("%-18s %-40s %-30s %s\n", r.Method, r.Path, r.Name, authStr)
	}
}

func printRoutesJSON(routes []RouteEntry) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(routes); err != nil {
		log.Fatalf("failed to encode JSON: %v", err)
	}
}

