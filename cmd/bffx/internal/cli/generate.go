package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/generator/mobile"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func HandleGenerate(args []string) {
	if len(args) < 1 {
		Usage()
		return
	}

	kind := args[0]
	name := ""
	if len(args) > 1 {
		name = args[1]
	}
	if kind == "client" {
		for _, a := range args[1:] {
			if a == "--help" || a == "-h" || a == "help" {
				printClientHelp()
				return
			}
		}
		if name == "" {
			printClientHelp()
			return
		}
	} else if kind != "openapi" && kind != "cicd" && name == "" {
		Usage()
		return
	}
	root := "."

	var fields []generator.Field
	var err error
	var feature string
	withHooks := true
	navType := "bottom"
	icon := ""
	order := 0
	requiresAuth := ""
	var pipelineName string
	var catalogName string
	readPolicy := ""
	writePolicy := ""

	actArgs := []string{}
	outputDir := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		} else if args[i] == "--out" && i+1 < len(args) {
			outputDir = args[i+1]
			i++
		} else if args[i] == "--feature" && i+1 < len(args) {
			feature = args[i+1]
			i++
		} else if args[i] == "--no-hooks" {
			withHooks = false
		} else if args[i] == "--no-admin-manifest" {
			// ignore, parsed globally
		} else if args[i] == "--nav" && i+1 < len(args) {
			navType = args[i+1]
			i++
		} else if args[i] == "--icon" && i+1 < len(args) {
			icon = args[i+1]
			i++
		} else if args[i] == "--order" && i+1 < len(args) {
			order, _ = strconv.Atoi(args[i+1])
			i++
		} else if args[i] == "--requires-auth" && i+1 < len(args) {
			requiresAuth = args[i+1]
			i++
		} else if args[i] == "--name" && i+1 < len(args) {
			pipelineName = args[i+1]
			i++
		} else if args[i] == "--catalog" && i+1 < len(args) {
			catalogName = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--catalog=") {
			catalogName = strings.TrimPrefix(args[i], "--catalog=")
		} else if args[i] == "--read-policy" && i+1 < len(args) {
			readPolicy = args[i+1]
			i++
		} else if args[i] == "--write-policy" && i+1 < len(args) {
			writePolicy = args[i+1]
			i++
		} else {
			actArgs = append(actArgs, args[i])
		}
	}
	args = actArgs

	if kind == "resource" || kind == "scaffold" {
		if len(args) < 3 {
			fmt.Println("usage: bffx generate " + kind + " NAME field:type ...")
			return
		}
		fields, err = generator.ParseFields(args[2:])
		if err != nil {
			log.Fatalf("invalid fields: %v", err)
		}
	}

	reg, _ := manifest.LoadAll(root)
	layout := generator.LayoutLegacy
	if reg != nil && reg.Project != nil {
		if reg.ProjectSpec().Layout == "v2" {
			layout = generator.LayoutV2
		}
	}

	switch kind {
	case "resource":
		err = generator.GenerateResource(root, name, fields, generator.ResourceOptions{
			WithHooks:   withHooks,
			Layout:      layout,
			Group:       feature,
			ReadPolicy:  readPolicy,
			WritePolicy: writePolicy,
		})
	case "scaffold":
		err = generator.GenerateScaffold(root, name, fields, generator.ResourceOptions{
			WithHooks:   withHooks,
			Layout:      layout,
			Group:       feature,
			ReadPolicy:  readPolicy,
			WritePolicy: writePolicy,
		})
	case "pipeline":
		pipelineType := name
		if pipelineName == "" {
			pipelineName = "ai_pipeline"
		}
		err = generator.GeneratePipeline(root, pipelineName, feature, pipelineType, catalogName, layout)
	case "builder":
		err = generator.GenerateBuilder(root, name, feature, layout)
	case "screen":
		err = generator.GenerateScreen(root, name, generator.ScreenOptions{
			NavType:      navType,
			Icon:         icon,
			Order:        order,
			RequiresAuth: requiresAuth,
			Group:        feature,
			Layout:       layout,
		})
	case "action":
		err = generator.GenerateAction(root, name, feature, layout)
	case "skill":
		err = generator.GenerateSkill(root, name)
	case "function":
		err = generator.GenerateFunction(root, name)
	case "service":
		err = generator.GenerateService(root, name, feature, "", layout)
	case "stream":
		err = generator.GenerateStream(root, name, feature, layout)
	case "template":
		err = generator.GenerateTemplate(root, name, feature, layout)
	case "cronjob":
		err = generator.GenerateCronJob(root, name, feature, layout)
	case "config":
		err = generator.GenerateConfig(root, name, args[2])
	case "openapi":
		spec, errBuild := compiler.BuildOpenAPI31(reg)
		if errBuild != nil {
			log.Fatalf("generate openapi failed: %v", errBuild)
		}
		targetFile := filepath.Join(root, ".bffx", "openapi.json")
		if outputDir != "" {
			targetFile = outputDir
		}
		if errMkdir := os.MkdirAll(filepath.Dir(targetFile), 0o755); errMkdir != nil {
			log.Fatalf("create dir failed: %v", errMkdir)
		}
		if errWrite := os.WriteFile(targetFile, spec, 0o644); errWrite != nil {
			log.Fatalf("write openapi failed: %v", errWrite)
		}
		fmt.Printf("Generated OpenAPI 3.1.0 specification at %s\n", targetFile)
		return
	case "client":
		platform := "flutter"
		if len(actArgs) > 1 {
			platform = actArgs[1]
		}
		if platform == "--help" || platform == "-h" || platform == "help" {
			printClientHelp()
			return
		}
		pLower := strings.ToLower(platform)
		if pLower == "flutter" || pLower == "dart" {
			err = mobile.GenerateFlutterClient(root, reg, outputDir)
		} else if pLower == "kotlin" || pLower == "android" {
			err = mobile.GenerateKotlinClient(root, reg)
		} else if pLower == "csharp" || pLower == "dotnet" {
			err = mobile.GenerateCSharpClient(root, reg)
		} else if pLower == "react-native" || pLower == "typescript" || pLower == "ts" || pLower == "js" {
			err = mobile.GenerateTypeScriptClient(root, reg)
		} else if pLower == "swift" || pLower == "ios" {
			err = generator.GenerateClientIOS(root, "ios")
		} else if pLower == "openapi" {
			targetFile := filepath.Join(root, ".bffx", "openapi.json")
			if outputDir != "" {
				targetFile = outputDir
			}
			spec, errBuild := compiler.BuildOpenAPI31(reg)
			if errBuild != nil {
				log.Fatalf("generate openapi failed: %v", errBuild)
			}
			_ = os.MkdirAll(filepath.Dir(targetFile), 0o755)
			if errWrite := os.WriteFile(targetFile, spec, 0o644); errWrite != nil {
				log.Fatalf("write openapi failed: %v", errWrite)
			}
			fmt.Printf("Generated OpenAPI 3.1.0 specification at %s\n", targetFile)
			return
		} else {
			err = fmt.Errorf("unsupported client platform: %s (supported: flutter, dart, typescript, react-native, kotlin, csharp, swift, openapi)", platform)
		}
	case "cicd":
		err = generator.GenerateCICD(root)
	default:
		log.Fatalf("unknown target %q", kind)
	}

	if err != nil {
		log.Fatalf("generate failed: %v", err)
	}

	fmt.Printf("generated %s %s\n", kind, name)
	if _, err := compiler.Sync(root, false); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
}

func printClientHelp() {
	fmt.Println("Usage:")
	fmt.Println("  bffx generate client [platform] [options]")
	fmt.Println("")
	fmt.Println("Supported client platforms:")
	fmt.Println("  flutter (or dart)    - Flutter / Dart client SDK")
	fmt.Println("  typescript (or ts)   - TypeScript client SDK")
	fmt.Println("  react-native         - React Native client SDK")
	fmt.Println("  kotlin (or android)  - Kotlin (Android) client SDK")
	fmt.Println("  csharp (or dotnet)   - C# / .NET client SDK")
	fmt.Println("  swift (or ios)       - Swift / iOS client SDK")
	fmt.Println("  openapi              - OpenAPI 3.1.0 specification (.bffx/openapi.json)")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --out <dir|file>     - Custom output directory or file path")
	fmt.Println("  --root <dir>         - BFFX project root (default: \".\")")
	fmt.Println("  --help, -h           - Show this help message")
}

