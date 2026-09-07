package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestSubscription(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("Subscription Resource", func() {
		bffxtest.It("should build a valid Subscription from blueprint", func() {
			data := env.Factory.Build("SubscriptionDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a Subscription in the store", func() {
			record, err := env.Factory.Create("SubscriptionDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
