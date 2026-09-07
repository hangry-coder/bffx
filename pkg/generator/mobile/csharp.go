package mobile

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateCSharpClient(root string, reg *manifest.Registry) error {
	clientDir := filepath.Join(root, "clients", "csharp", "Bffx.Gen")
	os.MkdirAll(clientDir, 0o755)

	var sb strings.Builder
	sb.WriteString("// GENERATED CODE - DO NOT MODIFY BY HAND\n")
	sb.WriteString("using System;\n")
	sb.WriteString("using System.Collections.Generic;\n")
	sb.WriteString("using System.Text.Json.Serialization;\n\n")
	sb.WriteString("namespace Bffx.Gen\n{\n")

	// 1. Generate Models
	for _, res := range reg.Resources {
		spec := reg.ResourceSpec(res.Metadata.Name)
		sb.WriteString(fmt.Sprintf("    public class %s\n    {\n", res.Metadata.Name))
		sb.WriteString("        [JsonPropertyName(\"id\")]\n")
		sb.WriteString("        public string Id { get; set; }\n\n")
		for _, f := range spec.Fields {
			csType := "string"
			if f.Type == "bool" {
				csType = "bool"
			} else if f.Type == "int" {
				csType = "int"
			}
			sb.WriteString(fmt.Sprintf("        [JsonPropertyName(\"%s\")]\n", f.Name))
			sb.WriteString(fmt.Sprintf("        public %s %s { get; set; }\n\n", csType, strings.Title(f.Name)))
		}
		sb.WriteString("    }\n\n")
	}

	// 2. Generate Base Client
	proj := reg.ProjectSpec()
	sb.WriteString("    public class BffxClient\n    {\n")
	sb.WriteString(fmt.Sprintf("        private const string BaseUrl = \"http://localhost:%d%s\";\n", proj.Runtime.Api.Port, proj.App.ApiPrefix))
	sb.WriteString("        private string _token;\n")
	sb.WriteString("        private string _deviceId;\n\n")
	sb.WriteString("        public void SetAuth(string token) => _token = token;\n")
	sb.WriteString("        public void SetDeviceId(string id) => _deviceId = id;\n\n")
	sb.WriteString("        // Add logic for HttpClient and JSON serialization\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")

	outputFile := filepath.Join(clientDir, "Models.gen.cs")
	return os.WriteFile(outputFile, []byte(sb.String()), 0o644)
}
