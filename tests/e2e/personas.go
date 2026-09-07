package e2e

import (
	"os"
)

type PersonaConfig struct {
	Name        string
	YAML        string
	RequiresEnv []string
}

func (p PersonaConfig) IsAvailable() bool {
	for _, env := range p.RequiresEnv {
		if os.Getenv(env) == "" {
			return false
		}
	}
	return true
}

func PersonaEnterpriseHeavy() PersonaConfig {
	return PersonaConfig{
		Name: "EnterpriseHeavy",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: enterprise-heavy
spec:
  runtime:
    worker: { enabled: true, language: python }
    redis: { enabled: true, url: "redis://localhost:6379" }
    streaming: { enabled: true }
  store: { mode: postgres, url: "${BFFX_TEST_POSTGRES_URL}" }
  telemetryStore: { mode: mongo, url: "${BFFX_TEST_MONGO_URL}" }
`,
		RequiresEnv: []string{"BFFX_TEST_POSTGRES_URL", "BFFX_TEST_MONGO_URL"},
	}
}

func PersonaStandardStartup() PersonaConfig {
	return PersonaConfig{
		Name: "StandardStartup",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: standard-startup
spec:
  runtime:
    worker: { enabled: true, language: python }
    redis: { enabled: true, url: "redis://localhost:6379" }
  store: { mode: postgres, url: "${BFFX_TEST_POSTGRES_URL}" }
  telemetryStore: { mode: postgres, url: "${BFFX_TEST_POSTGRES_URL}" }
`,
		RequiresEnv: []string{"BFFX_TEST_POSTGRES_URL"},
	}
}

func PersonaLightweightMonolith() PersonaConfig {
	return PersonaConfig{
		Name: "LightweightMonolith",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: lightweight-monolith
spec:
  runtime:
    worker: { enabled: false }
    redis: { enabled: false }
  store: { mode: sqlite, path: "app.db" }
  telemetryStore: { mode: sqlite, path: "telemetry.db" }
`,
	}
}

func PersonaEphemeral() PersonaConfig {
	return PersonaConfig{
		Name: "Ephemeral",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: ephemeral
spec:
  store: { mode: memory }
`,
	}
}

func PersonaNoSQLNative() PersonaConfig {
	return PersonaConfig{
		Name: "NoSQLNative",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: nosql-native
spec:
  runtime:
    worker: { enabled: true, language: python }
    redis: { enabled: true, url: "redis://localhost:6379" }
  store: { mode: mongo, url: "${BFFX_TEST_MONGO_URL}" }
  telemetryStore: { mode: mongo, url: "${BFFX_TEST_MONGO_URL}" }
`,
		RequiresEnv: []string{"BFFX_TEST_MONGO_URL"},
	}
}

func PersonaBFFExternal() PersonaConfig {
	return PersonaConfig{
		Name: "BFFExternal",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: bff-external
spec:
  runtime:
    worker: { enabled: false }
    redis: { enabled: false }
  store: { mode: postgres, url: "${BFFX_TEST_POSTGRES_URL}" }
`,
		RequiresEnv: []string{"BFFX_TEST_POSTGRES_URL"},
	}
}

func PersonaStatelessGateway() PersonaConfig {
	return PersonaConfig{
		Name: "StatelessGateway",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: stateless-gateway
spec:
  runtime:
    worker: { enabled: false }
    redis: { enabled: true, url: "redis://localhost:6379" }
  store: { mode: memory }
`,
		RequiresEnv: []string{"BFFX_TEST_REDIS_URL"},
	}
}

func PersonaHybridMobile() PersonaConfig {
	return PersonaConfig{
		Name: "HybridMobile",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: hybrid-mobile
spec:
  runtime:
    worker: { enabled: true, language: python }
    redis: { enabled: true, url: "redis://localhost:6379" }
  store: { mode: sqlite, path: "app.db" }
`,
		RequiresEnv: []string{"BFFX_TEST_REDIS_URL"},
	}
}

func PersonaAnalyticsPipeline() PersonaConfig {
	return PersonaConfig{
		Name: "AnalyticsPipeline",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: analytics-pipeline
spec:
  runtime:
    worker: { enabled: true, language: python }
    redis: { enabled: true, url: "redis://localhost:6379" }
  store: { mode: mongo, url: "${BFFX_TEST_MONGO_URL}" }
  telemetryStore: { mode: postgres, url: "${BFFX_TEST_POSTGRES_URL}" }
`,
		RequiresEnv: []string{"BFFX_TEST_MONGO_URL", "BFFX_TEST_POSTGRES_URL"},
	}
}

func PersonaServerlessCompute() PersonaConfig {
	return PersonaConfig{
		Name: "ServerlessCompute",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: serverless-compute
spec:
  runtime:
    worker: { enabled: true, language: python }
  store: { mode: memory }
  telemetryStore: { mode: memory }
`,
	}
}

func PersonaEdgeDeployment() PersonaConfig {
	return PersonaConfig{
		Name: "EdgeDeployment",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: edge-deployment
spec:
  store: { mode: sqlite, path: "edge.db" }
  telemetryStore: { mode: memory }
`,
	}
}

func PersonaMultiTenantSaaS() PersonaConfig {
	return PersonaConfig{
		Name: "MultiTenantSaaS",
		YAML: `
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: multi-tenant-saas
spec:
  runtime:
    worker: { enabled: true, language: python }
    redis: { enabled: true, url: "redis://localhost:6379" }
  store: { mode: postgres, url: "${BFFX_TEST_POSTGRES_URL}" }
  telemetryStore: { mode: sqlite, path: "tenant-logs.db" }
`,
		RequiresEnv: []string{"BFFX_TEST_POSTGRES_URL"},
	}
}

func AllPersonas() []PersonaConfig {
	return []PersonaConfig{
		PersonaEnterpriseHeavy(),
		PersonaStandardStartup(),
		PersonaNoSQLNative(),
		PersonaLightweightMonolith(),
		PersonaBFFExternal(),
		PersonaStatelessGateway(),
		PersonaHybridMobile(),
		PersonaAnalyticsPipeline(),
		PersonaServerlessCompute(),
		PersonaEdgeDeployment(),
		PersonaMultiTenantSaaS(),
		PersonaEphemeral(),
	}
}

func LocalPersonas() []PersonaConfig {
	var local []PersonaConfig
	for _, p := range AllPersonas() {
		if len(p.RequiresEnv) == 0 {
			local = append(local, p)
		}
	}
	return local
}
