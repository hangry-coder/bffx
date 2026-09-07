package mobile

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateKotlinClient(root string, reg *manifest.Registry) error {
	clientDir := filepath.Join(root, "clients", "kotlin", "src", "main", "kotlin", "com", "bffx", "gen")
	os.MkdirAll(clientDir, 0o755)

	var sb strings.Builder
	sb.WriteString("// GENERATED CODE - DO NOT MODIFY BY HAND\n")
	sb.WriteString("package com.bffx.gen\n\n")
	sb.WriteString("import kotlinx.serialization.Serializable\n")
	sb.WriteString("import kotlinx.serialization.json.Json\n\n")

	// 1. Generate Models
	for _, res := range reg.Resources {
		spec := reg.ResourceSpec(res.Metadata.Name)
		sb.WriteString("@Serializable\n")
		sb.WriteString(fmt.Sprintf("data class %s(\n", res.Metadata.Name))
		sb.WriteString("    val id: String,\n")
		for _, f := range spec.Fields {
			ktType := "String"
			if f.Type == "bool" {
				ktType = "Boolean"
			} else if f.Type == "int" {
				ktType = "Int"
			}
			sb.WriteString(fmt.Sprintf("    val %s: %s? = null,\n", f.Name, ktType))
		}
		sb.WriteString(")\n\n")
	}

	// 2. Generate Base Client (Conceptual)
	proj := reg.ProjectSpec()
	sb.WriteString("class BFFXClient(val baseUrl: String = \"http://localhost:%d%s\") {\n")
	sb.WriteString("    private var token: String? = null\n")
	sb.WriteString("    private var deviceId: String? = null\n\n")
	sb.WriteString("    fun setAuth(token: String) { this.token = token }\n")
	sb.WriteString("    fun setDeviceId(id: String) { this.deviceId = id }\n\n")
	sb.WriteString("    // Add networking library of choice (Ktor/Retrofit) logic here\n")
	sb.WriteString("}\n")

	outputFile := filepath.Join(clientDir, "BffxClient.gen.kt")
	return os.WriteFile(outputFile, []byte(fmt.Sprintf(sb.String(), proj.Runtime.Api.Port, proj.App.ApiPrefix)), 0o644)
}
