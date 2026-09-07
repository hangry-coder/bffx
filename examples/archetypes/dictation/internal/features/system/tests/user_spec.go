package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestUser(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("User Resource", func() {
		bffxtest.It("should build a valid User from blueprint", func() {
			data := env.Factory.Build("UserDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a User in the store", func() {
			record, err := env.Factory.Create("UserDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
