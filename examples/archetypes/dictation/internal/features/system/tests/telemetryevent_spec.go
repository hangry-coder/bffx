package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestTelemetryEvent(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("TelemetryEvent Resource", func() {
		bffxtest.It("should build a valid TelemetryEvent from blueprint", func() {
			data := env.Factory.Build("TelemetryEventDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a TelemetryEvent in the store", func() {
			record, err := env.Factory.Create("TelemetryEventDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
