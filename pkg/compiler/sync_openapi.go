package compiler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

// BuildOpenAPI31 exports a full OpenAPI 3.1.0 document representing the BFFX registry.
func BuildOpenAPI31(reg *manifest.Registry) ([]byte, error) {
	apiPrefix := reg.ApiPrefix
	if apiPrefix == "" {
		apiPrefix = "/api/v1"
	}

	port := 8080
	projName := "BFFX API"
	version := "1.0.0"
	if reg.Project != nil {
		spec := reg.ProjectSpec()
		if spec != nil {
			if spec.Runtime.Api.Port > 0 {
				port = spec.Runtime.Api.Port
			}
			if spec.App.MinClientVersion != "" {
				version = spec.App.MinClientVersion
			}
		}
		if reg.Project.Metadata.Name != "" {
			projName = reg.Project.Metadata.Name
		}
	}

	tags := []map[string]any{
		{"name": "System", "description": "Health checks, diagnostics, and app bootstrap"},
		{"name": "Auth", "description": "Authentication and session management"},
		{"name": "Resources", "description": "Entity CRUD operations"},
		{"name": "Screens", "description": "Backend-Driven UI screens and section endpoints"},
		{"name": "Streams", "description": "Real-time Server-Sent Events (SSE) feeds"},
		{"name": "Actions", "description": "Custom business logic and mutations"},
		{"name": "Pipelines", "description": "AI agent workflows and execution pipelines"},
	}

	servers := []map[string]any{
		{
			"url":         fmt.Sprintf("http://localhost:%d%s", port, apiPrefix),
			"description": "Local development server",
		},
	}

	paths := map[string]any{
		"/health": map[string]any{
			"get": map[string]any{
				"tags":        []string{"System"},
				"summary":     "Health check",
				"operationId": "getHealth",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Service is healthy",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"status": map[string]any{"type": "string", "example": "ok"},
									},
								},
							},
						},
					},
				},
			},
		},
		fmt.Sprintf("%s/app/bootstrap", apiPrefix): map[string]any{
			"get": map[string]any{
				"tags":        []string{"System"},
				"summary":     "App bootstrap payload",
				"operationId": "getAppBootstrap",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Active feature flags, client configuration, and screen manifests",
					},
				},
			},
		},
		fmt.Sprintf("%s/realtime", apiPrefix): map[string]any{
			"get": map[string]any{
				"tags":        []string{"Streams"},
				"summary":     "Universal real-time event stream",
				"operationId": "subscribeRealtime",
				"parameters": []map[string]any{
					{
						"name":        "channel",
						"in":          "query",
						"required":    false,
						"description": "Channel name or topic to subscribe to (e.g. bffx:events:users or posts)",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Continuous Server-Sent Events stream",
						"content": map[string]any{
							"text/event-stream": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
		fmt.Sprintf("%s/auth/login", apiPrefix): map[string]any{
			"post": map[string]any{
				"tags":        []string{"Auth"},
				"summary":     "User login",
				"operationId": "authLogin",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type":     "object",
								"required": []string{"email", "password"},
								"properties": map[string]any{
									"email":    map[string]any{"type": "string", "format": "email"},
									"password": map[string]any{"type": "string", "format": "password"},
								},
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{"description": "JWT session token and user profile"},
					"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
			},
		},
		fmt.Sprintf("%s/auth/signup", apiPrefix): map[string]any{
			"post": map[string]any{
				"tags":        []string{"Auth"},
				"summary":     "User registration",
				"operationId": "authSignup",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type":     "object",
								"required": []string{"email", "password"},
								"properties": map[string]any{
									"email":    map[string]any{"type": "string", "format": "email"},
									"password": map[string]any{"type": "string", "format": "password"},
									"name":     map[string]any{"type": "string"},
								},
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{"description": "User created successfully"},
					"400": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
			},
		},
		fmt.Sprintf("%s/auth/register", apiPrefix): map[string]any{
			"post": map[string]any{
				"tags":        []string{"Auth"},
				"summary":     "User registration (alias)",
				"operationId": "authRegister",
				"responses": map[string]any{
					"200": map[string]any{"description": "User created successfully"},
				},
			},
		},
		fmt.Sprintf("%s/auth/logout", apiPrefix): map[string]any{
			"post": map[string]any{
				"tags":        []string{"Auth"},
				"summary":     "User logout",
				"operationId": "authLogout",
				"security":    []any{map[string]any{"bearerAuth": []any{}}},
				"responses": map[string]any{
					"200": map[string]any{"description": "Session revoked"},
				},
			},
		},
		fmt.Sprintf("%s/auth/refresh", apiPrefix): map[string]any{
			"post": map[string]any{
				"tags":        []string{"Auth"},
				"summary":     "Rotate and refresh access token",
				"operationId": "authRefresh",
				"responses": map[string]any{
					"200": map[string]any{"description": "New access and refresh tokens"},
					"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
			},
		},
		fmt.Sprintf("%s/auth/anonymous", apiPrefix): map[string]any{
			"post": map[string]any{
				"tags":        []string{"Auth"},
				"summary":     "Anonymous guest session",
				"operationId": "authAnonymous",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type":     "object",
								"required": []string{"device_id"},
								"properties": map[string]any{
									"device_id": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{"description": "Guest session issued"},
				},
			},
		},
	}

	schemas := map[string]any{
		"Error": map[string]any{
			"type": "object",
			"required": []string{
				"code",
				"message",
			},
			"properties": map[string]any{
				"code":       map[string]any{"type": "string"},
				"message":    map[string]any{"type": "string"},
				"request_id": map[string]any{"type": "string"},
			},
		},
		"Event": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resource":  map[string]any{"type": "string"},
				"action":    map[string]any{"type": "string", "enum": []string{"created", "updated", "deleted"}},
				"payload":   map[string]any{"type": "object"},
				"timestamp": map[string]any{"type": "string", "format": "date-time"},
			},
		},
	}

	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		_ = r.UnmarshalSpec(&spec)

		name := r.Metadata.Name
		plural := apiPrefix + "/" + strings.ToLower(name) + "s"

		properties := map[string]any{
			"id": map[string]any{"type": "string"},
		}
		var requiredFields []string

		for _, f := range spec.Fields {
			if f.Name == "password" || f.Name == "otp_code" || strings.Contains(f.Name, "secret") {
				continue
			}

			t := "string"
			switch f.Type {
			case "int":
				t = "integer"
			case "float":
				t = "number"
			case "bool":
				t = "boolean"
			}

			if f.Required {
				properties[f.Name] = map[string]any{"type": t}
				requiredFields = append(requiredFields, f.Name)
			} else {
				// OpenAPI 3.1 standard nullable representation
				properties[f.Name] = map[string]any{"type": []string{t, "null"}}
			}
		}

		schema := map[string]any{
			"type":       "object",
			"properties": properties,
		}
		if len(requiredFields) > 0 {
			schema["required"] = requiredFields
		}
		schemas[name] = schema

		requestBody := map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"$ref": "#/components/schemas/" + name},
				},
			},
		}

		itemResponse := map[string]any{
			"200": map[string]any{
				"description": fmt.Sprintf("%s detail", name),
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/" + name},
					},
				},
			},
			"400": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
			"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
			"404": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
		}

		listResponse := map[string]any{
			"200": map[string]any{
				"description": fmt.Sprintf("List of %s entities", name),
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"type": "array",
							"items": map[string]any{
								"$ref": "#/components/schemas/" + name,
							},
						},
					},
				},
			},
			"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
		}

		paths[plural] = map[string]any{
			"get": map[string]any{
				"tags":        []string{"Resources"},
				"summary":     "List " + name,
				"operationId": "list" + name + "s",
				"parameters": []map[string]any{
					{"name": "limit", "in": "query", "description": "Maximum records to return", "schema": map[string]any{"type": "integer", "default": 50}},
					{"name": "offset", "in": "query", "description": "Number of records to skip", "schema": map[string]any{"type": "integer", "default": 0}},
					{"name": "sort", "in": "query", "description": "Sort order expression", "schema": map[string]any{"type": "string"}},
					{"name": "filter", "in": "query", "description": "Filter expression", "schema": map[string]any{"type": "string"}},
				},
				"responses": listResponse,
				"security":  []any{map[string]any{"bearerAuth": []any{}}},
			},
			"post": map[string]any{
				"tags":        []string{"Resources"},
				"summary":     "Create " + name,
				"operationId": "create" + name,
				"requestBody": requestBody,
				"responses": map[string]any{
					"201": map[string]any{
						"description": fmt.Sprintf("%s created successfully", name),
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/" + name},
							},
						},
					},
					"400": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
					"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}

		idParam := map[string]any{
			"name":        "id",
			"in":          "path",
			"required":    true,
			"description": "Unique record identifier",
			"schema":      map[string]any{"type": "string"},
		}

		paths[plural+"/{id}"] = map[string]any{
			"get": map[string]any{
				"tags":        []string{"Resources"},
				"summary":     "Get " + name,
				"operationId": "get" + name,
				"parameters":  []map[string]any{idParam},
				"responses":   itemResponse,
				"security":    []any{map[string]any{"bearerAuth": []any{}}},
			},
			"patch": map[string]any{
				"tags":        []string{"Resources"},
				"summary":     "Update " + name,
				"operationId": "update" + name,
				"parameters":  []map[string]any{idParam},
				"requestBody": requestBody,
				"responses":   itemResponse,
				"security":    []any{map[string]any{"bearerAuth": []any{}}},
			},
			"delete": map[string]any{
				"tags":        []string{"Resources"},
				"summary":     "Delete " + name,
				"operationId": "delete" + name,
				"parameters":  []map[string]any{idParam},
				"responses": map[string]any{
					"204": map[string]any{"description": fmt.Sprintf("%s deleted", name)},
					"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
					"404": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}
	}

	for _, a := range reg.Actions {
		var spec manifest.ActionSpec
		_ = a.UnmarshalSpec(&spec)

		path := spec.Route.Path
		method := strings.ToLower(spec.Route.Method)
		if method == "" {
			method = "post"
		}

		actionSchema := map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		}

		paths[path] = map[string]any{
			method: map[string]any{
				"tags":        []string{"Actions"},
				"summary":     a.Metadata.Name,
				"operationId": "action_" + a.Metadata.Name,
				"requestBody": map[string]any{
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": actionSchema,
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Successful action execution",
					},
					"400": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
					"401": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}
	}

	for _, p := range reg.Pipelines {
		var spec manifest.PipelineSpec
		_ = p.UnmarshalSpec(&spec)

		path := spec.Route.Path
		method := strings.ToLower(spec.Route.Method)
		if method == "" {
			method = "post"
		}

		paths[path] = map[string]any{
			method: map[string]any{
				"tags":        []string{"Pipelines"},
				"summary":     "AI Pipeline: " + p.Metadata.Name,
				"operationId": "pipeline_" + p.Metadata.Name,
				"requestBody": map[string]any{
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type":                 "object",
								"additionalProperties": true,
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Pipeline execution output",
					},
					"400": map[string]any{"$ref": "#/components/responses/ErrorResponse"},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}
	}

	for _, s := range reg.Screens {
		path := apiPrefix + "/screens/" + s.Metadata.Name
		paths[path] = map[string]any{
			"get": map[string]any{
				"tags":        []string{"Screens"},
				"summary":     "Get Screen " + s.Metadata.Name,
				"operationId": "getScreen_" + s.Metadata.Name,
				"responses": map[string]any{
					"200": map[string]any{"description": "Full success with all resolved sections"},
					"206": map[string]any{"description": "Partial success with degraded sections"},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}

		streamPath := path + "/stream"
		paths[streamPath] = map[string]any{
			"get": map[string]any{
				"tags":        []string{"Streams"},
				"summary":     "Stream Screen " + s.Metadata.Name,
				"operationId": "streamScreen_" + s.Metadata.Name,
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Server-Sent Events stream for screen updates",
						"content": map[string]any{
							"text/event-stream": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
						},
					},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}
	}

	for _, st := range reg.Streams {
		var spec manifest.StreamSpec
		_ = st.UnmarshalSpec(&spec)
		routePath := spec.Route.Path
		if routePath != "" {
			paths[routePath] = map[string]any{
				"get": map[string]any{
					"tags":        []string{"Streams"},
					"summary":     "Stream " + st.Metadata.Name,
					"operationId": "stream_" + st.Metadata.Name,
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Server-Sent Events channel stream",
							"content": map[string]any{
								"text/event-stream": map[string]any{
									"schema": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			}
		}
	}

	for _, b := range reg.Builders {
		path := apiPrefix + "/builders/" + b.Metadata.Name
		paths[path] = map[string]any{
			"get": map[string]any{
				"tags":        []string{"Screens"},
				"summary":     "Get Builder " + b.Metadata.Name,
				"operationId": "getBuilder_" + b.Metadata.Name,
				"responses": map[string]any{
					"200": map[string]any{"description": "Full builder output"},
					"206": map[string]any{"description": "Partial builder output"},
				},
				"security": []any{map[string]any{"bearerAuth": []any{}}},
			},
		}
	}

	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       projName,
			"version":     version,
			"description": "Modern API generated by BFFX framework with native realtime streams and CRUD resources.",
		},
		"servers": servers,
		"tags":    tags,
		"paths":   paths,
		"components": map[string]any{
			"schemas": schemas,
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
			"responses": map[string]any{
				"ErrorResponse": map[string]any{
					"description": "Standardized error response",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/Error"},
						},
					},
				},
			},
			"headers": map[string]any{
				"ClientVersion": map[string]any{
					"description": "Client version for compatibility checks",
					"schema":      map[string]any{"type": "string"},
				},
				"IdempotencyKey": map[string]any{
					"description": "Unique key to prevent duplicate operations",
					"schema":      map[string]any{"type": "string"},
				},
			},
		},
	}

	return json.MarshalIndent(doc, "", "  ")
}

func buildOpenAPI(reg *manifest.Registry) ([]byte, error) {
	return BuildOpenAPI31(reg)
}
