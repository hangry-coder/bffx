package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestUserInterest(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("UserInterest Resource", func() {
		bffxtest.It("should build a valid UserInterest from blueprint", func() {
			data := env.Factory.Build("UserInterestDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a UserInterest in the store", func() {
			record, err := env.Factory.Create("UserInterestDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
