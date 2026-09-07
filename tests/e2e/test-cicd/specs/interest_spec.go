package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestInterest(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("Interest Resource", func() {
		testing.It("should build a valid Interest from blueprint", func() {
			data := env.Factory.Build("InterestDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a Interest in the store", func() {
			record, err := env.Factory.Create("InterestDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
