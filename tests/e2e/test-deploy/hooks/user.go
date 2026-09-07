package hooks

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {
	emailAddr, _ := payload["email"].(string)
	userID, _ := payload["id"].(string)
	n, err := crand.Int(crand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	otp := fmt.Sprintf("%06d", n.Int64())
	ctx.Store.Update(ctx.Context, "User", userID, map[string]any{"otp_code": otp})
	return ctx.Comm.EmailMgr.Send(ctx.Context, "default", emailAddr, "Verify", "Code: "+otp)
}

func HandleVerify(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	var payload struct { Email string; Code string }
	json.NewDecoder(r.Body).Decode(&payload)
	errors.WriteJSON(w, http.StatusOK, map[string]any{"status": "verified"})
}