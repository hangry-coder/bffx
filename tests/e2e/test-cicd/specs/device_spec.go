package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestDevice(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("Device Resource", func() {
		testing.It("should build a valid Device from blueprint", func() {
			data := env.Factory.Build("DeviceDefault", nil)
			testing.Expect(data).ToNotEqual(nil)
		})

		testing.It("should create a Device in the store", func() {
			record, err := env.Factory.Create("DeviceDefault", nil)
			testing.Expect(err).ToEqual(nil)
			testing.Expect(record["id"]).ToNotEqual("")
		})
	})
}
