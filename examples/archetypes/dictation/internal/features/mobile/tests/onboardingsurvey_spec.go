package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestOnboardingSurvey(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("OnboardingSurvey Resource", func() {
		bffxtest.It("should build a valid OnboardingSurvey from blueprint", func() {
			data := env.Factory.Build("OnboardingSurveyDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a OnboardingSurvey in the store", func() {
			record, err := env.Factory.Create("OnboardingSurveyDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
