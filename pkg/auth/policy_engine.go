package auth

import (
	"fmt"
	"strings"
)

// PolicyEngine implements a declarative rule evaluator inspired by Pundit and CanCanCan.
// It supports simple keyword rules ("public", "owner", "authenticated"), 
// role-based checks, dynamic entitlement validation, and complex logical 
// operators ("and", "or", "unless").
type PolicyEngine struct{}

// NewPolicyEngine initializes a thread-safe evaluator for security policies.
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{}
}

// Evaluate checks if the current claims (user context) satisfy the policy rule for a specific resource.
func (e *PolicyEngine) Evaluate(rule any, claims map[string]any, resource map[string]any) bool {
	if rule == nil {
		return false
	}

	switch r := rule.(type) {
	case string:
		return e.evaluateString(r, claims, resource)
	case map[string]any:
		return e.evaluateMap(r, claims, resource)
	case []any:
		// Default to OR if it's just a list
		for _, sub := range r {
			if e.Evaluate(sub, claims, resource) {
				return true
			}
		}
		return false
	}

	return false
}

func isAdminClaims(claims map[string]any) bool {
	if claims == nil {
		return false
	}
	role, _ := claims["role"].(string)
	r := strings.ToLower(strings.TrimSpace(role))
	return r == "admin" || r == "administrator"
}

func (e *PolicyEngine) evaluateString(rule string, claims map[string]any, resource map[string]any) bool {
	rule = strings.ToLower(strings.TrimSpace(rule))

	switch rule {
	case "public":
		return true
	case "authenticated":
		return claims != nil
	case "owner":
		if isAdminClaims(claims) {
			return true
		}
		if claims == nil || resource == nil {
			return false
		}
		userID := fmt.Sprintf("%v", claims["sub"])
		createdBy := fmt.Sprintf("%v", resource["created_by"])
		resourceID := fmt.Sprintf("%v", resource["id"])
		return userID != "" && (userID == createdBy || userID == resourceID)
	case "admin", "administrator":
		if claims == nil {
			return false
		}
		role, _ := claims["role"].(string)
		return strings.ToLower(role) == "admin"
	}

	// Dynamic entitlement check: if rule is "entitled:pro", check if "pro" is in claims["entitlements"]
	if strings.HasPrefix(rule, "entitled:") {
		if isAdminClaims(claims) {
			return true
		}
		slug := strings.TrimPrefix(rule, "entitled:")
		if claims == nil {
			return false
		}
		entitlements, _ := claims["entitlements"].([]string)
		for _, e := range entitlements {
			if strings.ToLower(e) == strings.ToLower(slug) {
				return true
			}
		}
		return false
	}

	// Dynamic role check: if rule is "pro", check if claims["role"] == "pro"
	if claims != nil {
		if isAdminClaims(claims) {
			return true
		}
		role, _ := claims["role"].(string)
		if strings.ToLower(role) == rule {
			return true
		}
	}

	return false
}

func (e *PolicyEngine) evaluateMap(rule map[string]any, claims map[string]any, resource map[string]any) bool {
	// Handle AND logic
	if andRules, ok := rule["and"].([]any); ok {
		for _, sub := range andRules {
			if !e.Evaluate(sub, claims, resource) {
				return false
			}
		}
		return true
	}

	// Handle OR logic
	if orRules, ok := rule["or"].([]any); ok {
		for _, sub := range orRules {
			if e.Evaluate(sub, claims, resource) {
				return true
			}
		}
		return false
	}

	// Handle UNLESS logic (rule passes if sub-rule fails)
	if unlessRule, ok := rule["unless"]; ok {
		return !e.Evaluate(unlessRule, claims, resource)
	}

	return false
}
