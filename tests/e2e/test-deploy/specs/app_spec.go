package specs

import (
	"github.com/hangry-coder/bffx/pkg/testing"
	stdtesting "testing"
)

func TestApp(t *stdtesting.T) {
	testing.SetT(t)
	env := testing.SetupEnvironment("..")

	testing.Describe("User Onboarding", func() {
		testing.It("creates a new user with free tier", func() {
			user, _ := env.Factory.Create("AdminUser", nil)
			testing.Expect(user["role"]).ToEqual("admin")
		})
	})
}
