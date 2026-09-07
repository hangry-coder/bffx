package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestFeatureFlag(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("FeatureFlag Resource", func() {
		bffxtest.It("should build a valid FeatureFlag from blueprint", func() {
			data := env.Factory.Build("FeatureFlagDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a FeatureFlag in the store", func() {
			record, err := env.Factory.Create("FeatureFlagDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
