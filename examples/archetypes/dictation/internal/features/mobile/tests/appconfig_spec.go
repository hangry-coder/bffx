package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestAppConfig(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("AppConfig Resource", func() {
		bffxtest.It("should build a valid AppConfig from blueprint", func() {
			data := env.Factory.Build("AppConfigDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a AppConfig in the store", func() {
			record, err := env.Factory.Create("AppConfigDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
