package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestAppString(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("AppString Resource", func() {
		testing.It("should build a valid AppString from blueprint", func() {
			data := env.Factory.Build("AppStringDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a AppString in the store", func() {
			record, err := env.Factory.Create("AppStringDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
