package specs

import (
	bffxtest "github.com/hangry-coder/bffx/pkg/testing"
	"testing"
)

func TestAuditLog(t *testing.T) {
	bffxtest.SetT(t)
	env := bffxtest.SetupEnvironment("..")

	bffxtest.Describe("AuditLog Resource", func() {
		bffxtest.It("should build a valid AuditLog from blueprint", func() {
			data := env.Factory.Build("AuditLogDefault", nil)
			bffxtest.Expect(data).ToNotEqual(nil)
		})

		bffxtest.It("should create a AuditLog in the store", func() {
			record, err := env.Factory.Create("AuditLogDefault", nil)
			bffxtest.Expect(err).ToEqual(nil)
			bffxtest.Expect(record["id"]).ToNotEqual("")
		})
	})
}
