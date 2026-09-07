package hooks

import (
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"fmt"
	"math/rand"
)

// SendOTP (Async) - Triggered AfterCreate
func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {
	emailVal, ok := payload["email"]
	if !ok || emailVal == nil {
		return nil
	}
	emailAddr, ok := emailVal.(string)
	if !ok || emailAddr == "" {
		return nil
	}
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))

	idVal, ok := payload["id"]
	if !ok || idVal == nil {
		return nil
	}
	userID, ok := idVal.(string)
	if !ok || userID == "" {
		return nil
	}

	ctx.Store.Update(ctx.Context, "User", userID, map[string]any{
		"otp_code": otp,
	})

	subject := "Verify your account"
	body := fmt.Sprintf("<h1>Welcome!</h1><p>Your verification code is: <b>%s</b></p>", otp)
	return ctx.Comm.EmailMgr.Send(ctx.Context, "default", emailAddr, subject, body)
}

// HandleRegistrationEffects (Sync) - AfterCreate
func HandleRegistrationEffects(ctx *handlers.ActionContext, payload map[string]any) error {
	idVal, ok := payload["id"]
	if !ok || idVal == nil {
		return nil
	}
	userID, ok := idVal.(string)
	if !ok || userID == "" {
		return nil
	}
	// 1. Auto-grant FREE tier subscription
	ctx.Store.Create(ctx.Context, "Subscription", map[string]any{
		"user_id": userID,
		"plan_id": "free",
		"status":  "active",
	})
	return nil
}

func CheckVerification(ctx *handlers.ActionContext, payload map[string]any) error {
	verified, _ := payload["is_verified"].(bool)
	if !verified { return nil }
	return nil
}

func SendWelcome(ctx *handlers.ActionContext, payload map[string]any) error {
	verified, _ := payload["is_verified"].(bool)
	if !verified { return nil }
	emailVal, ok := payload["email"]
	if !ok || emailVal == nil {
		return nil
	}
	emailAddr, ok := emailVal.(string)
	if !ok || emailAddr == "" {
		return nil
	}
	subject := "Welcome!"
	body := "<h1>Verified!</h1><p>Thanks for joining.</p>"
	return ctx.Comm.EmailMgr.Send(ctx.Context, "default", emailAddr, subject, body)
}

func HandleVerifyOTP(ctx *handlers.ActionContext, payload map[string]any) error {
	emailVal, ok := payload["email"]
	if !ok || emailVal == nil {
		return fmt.Errorf("email required")
	}
	email, ok := emailVal.(string)
	if !ok {
		return fmt.Errorf("invalid email")
	}
	codeVal, ok := payload["code"]
	if !ok || codeVal == nil {
		return fmt.Errorf("code required")
	}
	code, ok := codeVal.(string)
	if !ok {
		return fmt.Errorf("invalid code")
	}
	users, _ := ctx.Store.Query(ctx.Context, "User").Where("email", "==", email).Execute(ctx.Context)
	if len(users) == 0 { return fmt.Errorf("user not found") }
	user := users[0]
	if user["otp_code"] != code { return fmt.Errorf("invalid code") }
	idVal, ok := user["id"]
	if !ok || idVal == nil {
		return fmt.Errorf("invalid user record")
	}
	userID, ok := idVal.(string)
	if !ok || userID == "" {
		return fmt.Errorf("invalid user ID")
	}
	ctx.Store.Update(ctx.Context, "User", userID, map[string]any{
		"is_verified": true,
		"otp_code": "",
	})
	return nil
}