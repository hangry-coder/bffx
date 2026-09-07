package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestAppConfig(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("AppConfig Resource", func() {
		testing.It("should build a valid AppConfig from blueprint", func() {
			data := env.Factory.Build("AppConfigDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a AppConfig in the store", func() {
			record, err := env.Factory.Create("AppConfigDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
