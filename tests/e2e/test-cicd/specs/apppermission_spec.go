package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestAppPermission(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("AppPermission Resource", func() {
		testing.It("should build a valid AppPermission from blueprint", func() {
			data := env.Factory.Build("AppPermissionDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a AppPermission in the store", func() {
			record, err := env.Factory.Create("AppPermissionDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
