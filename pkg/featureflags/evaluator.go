package featureflags

import (
	// #nosec G505
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
)

// Evaluator handles the logic of resolving a flag for a context
type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate returns the variation value for the given flag and context
func (e *Evaluator) Evaluate(flag FlagDefinition, ctx EvalContext) interface{} {
	// 1. Global Kill Switch
	if !flag.Enabled {
		return e.getVariationValue(flag, flag.OffVariation)
	}

	// 2. Process Prerequisites
	for _, pre := range flag.Prerequisites {
		// Note: This requires a way to evaluate other flags. 
		// For now, we assume prerequisites are handled by the provider or caller.
		_ = pre 
	}

	// 2.5 Process simplified Targeting Rules (for dashboard rules)
	if flag.Targeting != nil {
		matched := false

		// Check user ID matching
		if len(flag.Targeting.Users) > 0 && ctx.UserID != "" {
			for _, u := range flag.Targeting.Users {
				if u == ctx.UserID {
					matched = true
					break
				}
			}
		}

		// Check segments matching
		if !matched && len(flag.Targeting.Segments) > 0 && len(ctx.Segments) > 0 {
			for _, s := range flag.Targeting.Segments {
				for _, cs := range ctx.Segments {
					if strings.EqualFold(s, cs) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}

		// Check percentage rollout matching
		if !matched && flag.Targeting.Rollout > 0 {
			bucketKey := ctx.UserID
			if bucketKey == "" {
				bucketKey = ctx.DeviceID
			}
			if bucketKey != "" {
				bucket := e.getBucket(flag.Key, bucketKey)
				if (bucket / 1000) < flag.Targeting.Rollout {
					matched = true
				}
			}
		}

		if matched {
			return e.getVariationValue(flag, 0)
		} else {
			return e.getVariationValue(flag, flag.OffVariation)
		}
	}

	// 3. Process Targeting Rules
	for _, rule := range flag.Rules {
		if e.matchRule(rule, ctx) {
			return e.selectVariation(rule.Variation, rule.Rollout, flag, ctx)
		}
	}

	// 4. Fallthrough
	return e.selectVariation(flag.Fallthrough.Variation, flag.Fallthrough.Rollout, flag, ctx)
}

func (e *Evaluator) matchRule(rule TargetingRule, ctx EvalContext) bool {
	if len(rule.Clauses) == 0 {
		return false
	}
	for _, clause := range rule.Clauses {
		if !e.matchClause(clause, ctx) {
			return false
		}
	}
	return true
}

func (e *Evaluator) matchClause(clause Clause, ctx EvalContext) bool {
	val := e.getAttribute(clause.Attribute, ctx)
	matched := false

	switch strings.ToLower(clause.Operator) {
	case "in":
		matched = e.contains(clause.Values, val)
	case "notin":
		matched = !e.contains(clause.Values, val)
	case "startswith":
		if s, ok := val.(string); ok {
			for _, v := range clause.Values {
				if vs, ok := v.(string); ok && strings.HasPrefix(s, vs) {
					matched = true
					break
				}
			}
		}
	case "endswith":
		if s, ok := val.(string); ok {
			for _, v := range clause.Values {
				if vs, ok := v.(string); ok && strings.HasSuffix(s, vs) {
					matched = true
					break
				}
			}
		}
	case "contains":
		if s, ok := val.(string); ok {
			for _, v := range clause.Values {
				if vs, ok := v.(string); ok && strings.Contains(s, vs) {
					matched = true
					break
				}
			}
		}
	case "matches":
		if s, ok := val.(string); ok {
			for _, v := range clause.Values {
				if vs, ok := v.(string); ok {
					if re, err := regexp.Compile(vs); err == nil && re.MatchString(s) {
						matched = true
						break
					}
				}
			}
		}
	// TODO: Add more operators (greaterThan, lessThan, semVer, etc.) in Phase 1.4
	}

	if clause.Negate {
		return !matched
	}
	return matched
}

func (e *Evaluator) getAttribute(attr string, ctx EvalContext) interface{} {
	switch strings.ToLower(attr) {
	case "id", "userid", "user_id":
		return ctx.UserID
	case "role", "userrole", "user_role":
		return ctx.UserRole
	case "deviceid", "device_id":
		return ctx.DeviceID
	case "email":
		return ctx.Email
	case "country":
		return ctx.Country
	case "platform":
		return ctx.Platform
	case "appversion":
		return ctx.AppVersion
	}
	if ctx.Attributes != nil {
		if v, ok := ctx.Attributes[attr]; ok {
			return v
		}
	}
	if ctx.Claims != nil {
		if v, ok := ctx.Claims[attr]; ok {
			return v
		}
	}
	return nil
}

func (e *Evaluator) contains(slice []interface{}, val interface{}) bool {
	for _, item := range slice {
		if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", val) {
			return true
		}
	}
	return false
}

func (e *Evaluator) selectVariation(variation *int, rollout *Rollout, flag FlagDefinition, ctx EvalContext) interface{} {
	if variation != nil {
		return e.getVariationValue(flag, *variation)
	}

	if rollout != nil {
		bucketBy := rollout.BucketBy
		if bucketBy == "" {
			bucketBy = "id"
		}
		attrVal := e.getAttribute(bucketBy, ctx)
		bucket := e.getBucket(flag.Key, fmt.Sprintf("%v", attrVal))
		
		var cumulativeWeight int
		for _, wv := range rollout.Variations {
			cumulativeWeight += wv.Weight
			if bucket < cumulativeWeight {
				return e.getVariationValue(flag, wv.Variation)
			}
		}
	}

	return nil
}

func (e *Evaluator) getVariationValue(flag FlagDefinition, index int) interface{} {
	if index >= 0 && index < len(flag.Variations) {
		return flag.Variations[index].Value
	}
	return nil
}

func (e *Evaluator) getBucket(flagKey, salt string) int {
	data := fmt.Sprintf("%s:%s", flagKey, salt)
	// #nosec G401
	h := sha1.New()
	h.Write([]byte(data))
	hash := h.Sum(nil)
	val := binary.BigEndian.Uint32(hash[:4])
	return int(val % 100000)
}

