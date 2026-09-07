package e2e

import (
	"fmt"
	"testing"
)

func TestSpecializedArchitectureMatrix(t *testing.T) {
	personas := []PersonaConfig{
		PersonaBFFExternal(),
		PersonaStatelessGateway(),
		PersonaHybridMobile(),
		PersonaAnalyticsPipeline(),
	}

	mutations := AllMutations()

	for _, p := range personas {
		if !p.IsAvailable() {
			t.Logf("Skipping persona %s (missing env vars)", p.Name)
			continue
		}

		for _, m := range mutations {
			t.Run(fmt.Sprintf("%s/%s", p.Name, m.Name), func(t *testing.T) {
				ts := NewTestServer(t, p, m)
				runCoreE2EAssertions(t, ts)
			})
		}
	}
}
