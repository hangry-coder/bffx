package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestPlan(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("Plan Resource", func() {
		bffxtest.It("should build a valid Plan from blueprint", func() {
			data := env.Factory.Build("PlanDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a Plan in the store", func() {
			record, err := env.Factory.Create("PlanDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
