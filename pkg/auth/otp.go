package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
	"golang.org/x/crypto/bcrypt"
)

type OTPService struct{}

func NewOTPService() *OTPService {
	return &OTPService{}
}

// Generate creates a random 6-digit code, its bcrypt hash, and an expiry time.
func (s *OTPService) Generate() (string, string, time.Time, error) {
	code := ""
	for i := 0; i < 6; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", "", time.Time{}, fmt.Errorf("crypto rand failed: %w", err)
		}
		code += fmt.Sprintf("%d", num)
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("bcrypt hash failed: %w", err)
	}
	
	// Valid for 5 minutes
	return code, string(hash), time.Now().Add(5 * time.Minute), nil
}

// Verify checks if the input code matches the hashed code.
func (s *OTPService) Verify(hashedCode string, inputCode string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(inputCode))
	return err == nil
}
