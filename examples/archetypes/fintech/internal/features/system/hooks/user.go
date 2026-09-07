package hooks

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"encoding/json"
	"net/http"
)

func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {
	emailAddr, _ := payload["email"].(string)
	userID, _ := payload["id"].(string)
	code, hashed, _, err := ctx.OTP.Generate()
	if err != nil {
		return err
	}
	ctx.Store.Update(ctx.Context, "User", userID, map[string]any{"otp_code": hashed})
	return ctx.Comm.EmailMgr.Send(ctx.Context, "default", emailAddr, "Verify", "Code: "+code)
}

func CheckVerification(ctx *handlers.ActionContext, payload map[string]any) error {
	return nil
}

func SendWelcome(ctx *handlers.ActionContext, payload map[string]any) error {
	return nil
}

func HandleConvertGuestToUser(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	errors.WriteJSON(w, http.StatusOK, map[string]any{"status": "converted"})
}

func HandleVerifyOTP(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	var payload struct { Email string; Code string }
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	user, err := ctx.Store.GetByField(ctx.Context, "User", "email", payload.Email)
	if err != nil {
		errors.Write(w, errors.ErrNotFound)
		return
	}
	hashedCode, _ := user["otp_code"].(string)
	if !ctx.OTP.Verify(hashedCode, payload.Code) {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}
	ctx.Store.Update(ctx.Context, "User", user["id"].(string), map[string]any{"otp_code": "", "is_verified": true})
	errors.WriteJSON(w, http.StatusOK, map[string]any{"status": "verified"})
}