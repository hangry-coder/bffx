package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestUser(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("User Resource", func() {
		testing.It("should build a valid User from blueprint", func() {
			data := env.Factory.Build("UserDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a User in the store", func() {
			record, err := env.Factory.Create("UserDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
