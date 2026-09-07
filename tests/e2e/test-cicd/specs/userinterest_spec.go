package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestUserInterest(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("UserInterest Resource", func() {
		testing.It("should build a valid UserInterest from blueprint", func() {
			data := env.Factory.Build("UserInterestDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a UserInterest in the store", func() {
			record, err := env.Factory.Create("UserInterestDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
