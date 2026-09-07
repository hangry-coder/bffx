package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestEntitlement(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("Entitlement Resource", func() {
		bffxtest.It("should build a valid Entitlement from blueprint", func() {
			data := env.Factory.Build("EntitlementDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a Entitlement in the store", func() {
			record, err := env.Factory.Create("EntitlementDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
