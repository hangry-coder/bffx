// Package handlers implements the generic REST API handlers for CRUD actions,
// health checking, file uploads, background jobs, telemetry routing, and system monitoring.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/hangry-coder/bffx/pkg/addons/ads"
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries/analytics"
	"github.com/hangry-coder/bffx/pkg/batteries/nutrition"
	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/game/liveops"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/api/sdui"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
)

type CommonHandler struct {
	reg              *manifest.Registry
	featureFlags     *featureflags.FlagService
	ads              *ads.AdService
	i18n             *i18n.Bundle
	store            storage.Store
	liveopsScheduler *liveops.Scheduler
}

func NewCommonHandler(reg *manifest.Registry, fs *featureflags.FlagService, as *ads.AdService, bundle *i18n.Bundle, store storage.Store, liveopsScheduler *liveops.Scheduler) *CommonHandler {
	return &CommonHandler{reg: reg, featureFlags: fs, ads: as, i18n: bundle, store: store, liveopsScheduler: liveopsScheduler}
}

func (h *CommonHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetClaims(ctx)
	locale := middleware.GetLocale(ctx)

	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "ios" // Default
	}

	// Fetch dynamic overrides from AppString resource if it exists
	overrides := h.getAppStringOverrides(ctx, locale)

	// Phase 1: Navigation from Screens
	var bottomNav []map[string]any
	var foldedMenu []map[string]any

	// Sort screens by order
	sort.Slice(h.reg.Screens, func(i, j int) bool {
		var specI, specJ manifest.ScreenSpec
		h.reg.Screens[i].UnmarshalSpec(&specI)
		h.reg.Screens[j].UnmarshalSpec(&specJ)
		return specI.Order < specJ.Order
	})

	for _, m := range h.reg.Screens {
		var spec manifest.ScreenSpec
		m.UnmarshalSpec(&spec)

		item := map[string]any{
			"name":          spec.Name,
			"icon":          spec.Icon,
			"requires_auth": spec.RequiresAuth,
		}

		if spec.NavType == "bottom" {
			bottomNav = append(bottomNav, item)
		} else if spec.NavType == "folded" {
			foldedMenu = append(foldedMenu, item)
		}
	}

	// Onboarding Config
	proj := h.reg.ProjectSpec()
	authStrategy := proj.App.AuthStrategy
	if authStrategy == "" {
		authStrategy = "optional"
	}

	onboardingSequence := []string{"intro_carousel", "permission_request", "home"}
	if authStrategy == "optional" || authStrategy == "mandatory" {
		onboardingSequence = []string{"intro_carousel", "auth_gateway", "personalization_step", "home"}
	}

	// Mock is_onboarded for now (in real app, check user/device record)
	isOnboarded := false
	if claims != nil {
		// Example: userID := claims["sub"].(string)
		// Check store for User.is_onboarded
	}

	var projectSpec manifest.ProjectSpec
	h.reg.Project.UnmarshalSpec(&projectSpec)
	
	minClientVersion := projectSpec.App.MinClientVersion
	if minClientVersion == "" {
		minClientVersion = "0.1.0"
	}

	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"app": map[string]any{
			"name":             h.reg.Project.Metadata.Name,
			"version":          "0.1.0",
			"apiPrefix":        "/api/v1",
			"minClientVersion": minClientVersion,
		},
		"auth": map[string]any{
			"enabled":     true,
			"requirement": authStrategy,
		},
		"onboarding": map[string]any{
			"skip_onboarding":     isOnboarded,
			"onboarding_sequence": onboardingSequence,
			"can_skip":            authStrategy == "optional",
		},
		"navigation": map[string]any{
			"bottom_nav":  bottomNav,
			"folded_menu": foldedMenu,
		},
		"features": map[string]any{
			"capabilities": []string{"auth", "files", "jobs", "builders", "streams", "trees", "billing", "ads", "liveops"},
			"flags": func() any {
				userID := middleware.GetUserID(ctx)
				deviceID := r.Header.Get("X-Device-ID")
				role := ""
				if claims != nil {
					if r, ok := claims["role"].(string); ok {
						role = r
					}
				}

				var segments []string
				if claims != nil {
					if segsVal, ok := claims["segments"]; ok {
						if list, ok := segsVal.([]any); ok {
							for _, item := range list {
								if s, ok := item.(string); ok {
									segments = append(segments, s)
								}
							}
						} else if list, ok := segsVal.([]string); ok {
							segments = list
						}
					}
				}

				fctx := featureflags.EvalContext{
					UserID:   userID,
					UserRole: role,
					DeviceID: deviceID,
					Claims:   claims,
					Segments: segments,
				}
				results := map[string]any{
					"flags": h.featureFlags.GetAllResolved(fctx),
					"screens": func() any {
						screenResults := make(map[string]any)
						for _, m := range h.reg.Screens {
							var spec manifest.ScreenSpec
							if err := m.UnmarshalSpec(&spec); err == nil {
								name := spec.Name
								if name == "" {
									name = m.Metadata.Name
								}
								screenResults[name] = h.featureFlags.ResolveScreenSections(name, fctx)
							}
						}
						return screenResults
					}(),
				}
				return results
			}(),
			"liveops": func() any {
				userID := middleware.GetUserID(ctx)
				deviceID := r.Header.Get("X-Device-ID")
				role := ""
				if claims != nil {
					if r, ok := claims["role"].(string); ok {
						role = r
					}
				}

				var segments []string
				if claims != nil {
					if segsVal, ok := claims["segments"]; ok {
						if list, ok := segsVal.([]any); ok {
							for _, item := range list {
								if s, ok := item.(string); ok {
									segments = append(segments, s)
								}
							}
						} else if list, ok := segsVal.([]string); ok {
							segments = list
						}
					}
				}

				fctx := featureflags.EvalContext{
					UserID:   userID,
					UserRole: role,
					DeviceID: deviceID,
					Claims:   claims,
					Segments: segments,
				}

				if h.liveopsScheduler != nil {
					active := h.liveopsScheduler.Evaluate(fctx, time.Now().UTC())
					type ClientLiveOpsEvent struct {
						Name                 string         `json:"name"`
						Title                string         `json:"title"`
						Description          string         `json:"description"`
						Priority             int            `json:"priority"`
						AudienceSegment      string         `json:"audience_segment"`
						ConfigurationPayload map[string]any `json:"configuration_payload"`
					}
					clientEvs := make([]ClientLiveOpsEvent, 0, len(active))
					for _, ev := range active {
						clientEvs = append(clientEvs, ClientLiveOpsEvent{
							Name:                 ev.Name,
							Title:                ev.Title,
							Description:          ev.Description,
							Priority:             ev.Priority,
							AudienceSegment:      ev.AudienceSegment,
							ConfigurationPayload: ev.ConfigurationPayload,
						})
					}
					return clientEvs
				}
				return []any{}
			}(),
		},
		"monetization": map[string]any{
			"ads": func() any {
				if h.ads != nil {
					return map[string]any{
						"units":     h.ads.ResolveAdUnits(platform),
						"providers": h.ads.GetProviders(),
					}
				}
				return map[string]any{"units": []any{}, "providers": []any{}}
			}(),
		},
		"translations": func() any {
			if h.i18n != nil {
				return h.i18n.GetMap(locale, overrides)
			}
			return map[string]string{}
		}(),
		"config": map[string]any{
			"permissions": h.resolvePermissions(ctx, proj.App.Permissions, locale),
			"ui":          proj.App.UI,
			"menu":        proj.App.Menu,
		},
	})
}

func (h *CommonHandler) resolvePermissions(ctx context.Context, perms []manifest.AppPermission, locale string) []map[string]any {
	resolved := make([]map[string]any, 0, len(perms))
	for _, p := range perms {
		justification := p.Justification
		if p.Rationales != nil {
			if val, ok := p.Rationales[locale]; ok {
				justification = val
			} else if val, ok := p.Rationales["en"]; ok {
				justification = val
			}
		}

		title := ""
		if p.Title != nil {
			if val, ok := p.Title[locale]; ok {
				title = val
			} else {
				title = p.Title["en"]
			}
		}

		description := ""
		if p.Description != nil {
			if val, ok := p.Description[locale]; ok {
				description = val
			} else {
				description = p.Description["en"]
			}
		}

		resolved = append(resolved, map[string]any{
			"type":            p.Type,
			"id":              p.ID,
			"title":           title,
			"description":     description,
			"required":        p.Required,
			"level":           p.Level,
			"timing":          p.Timing,
			"trigger_feature": p.TriggerFeature,
			"justification":   justification,
		})
	}
	return resolved
}

func (h *CommonHandler) UIRegistry(w http.ResponseWriter, r *http.Request) {
	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"registry": sdui.Registry,
	})
}

func (h *CommonHandler) getAppStringOverrides(ctx context.Context, locale string) map[string]string {
	overrides := make(map[string]string)
	if h.store == nil {
		return overrides
	}

	// Try to fetch AppString records for this locale
	res, err := h.store.Query(ctx, "AppString").Where("locale", "==", locale).Execute(ctx)
	if err != nil {
		return overrides
	}
	for _, item := range res {
		key, _ := item["key"].(string)
		val, _ := item["value"].(string)
		if key != "" && val != "" {
			overrides[key] = val
		}
	}
	return overrides
}

type ActionContext struct {
	Store     storage.Store
	Telemetry storage.Store // Dedicated store for analytics/high-volume data
	Auth      *auth.JWTService
	JobStore  worker.JobStore
	Queue     worker.Queue
	Notify    *notifications.Manager
	Comm      *comm.Hub
	Analytics analytics.Provider
	OTP       *auth.OTPService
	FeatureFlags *featureflags.FlagService
	Bundle    *i18n.Bundle
	Localizer *i18n.Localizer
	Vlm       vlm.Provider
	Nutrition nutrition.Provider

	Ads     *ads.AdService
	User    map[string]any // Current authenticated user
	Claims  map[string]any
	Context context.Context // Request context
	Request *http.Request   // Original request
}

func (c *ActionContext) UserID() string {
	if c.User != nil {
		if id, ok := c.User["id"].(string); ok {
			return id
		}
	}
	if c.Claims != nil {
		if sub, ok := c.Claims["sub"].(string); ok {
			return sub
		}
	}
	return ""
}

func (c *ActionContext) Flag(key string) interface{} {
	if c.FeatureFlags == nil {
		return nil
	}
	userID := c.UserID()
	deviceID := c.Request.Header.Get("X-Device-ID")
	role := ""
	if c.Claims != nil {
		if r, ok := c.Claims["role"].(string); ok {
			role = r
		}
	}

	fctx := featureflags.EvalContext{
		UserID:   userID,
		UserRole: role,
		DeviceID: deviceID,
		Claims:   c.Claims,
	}
	return c.FeatureFlags.GetValue(key, fctx)
}

func (c *ActionContext) T(key string) string {
	locale := middleware.GetLocale(c.Context)
	// For now, we don't pass dynamic overrides to every individual T() call in handlers
	// unless we implement a local override cache in ActionContext.
	return c.Bundle.Translate(locale, key, nil)
}

func (c *ActionContext) TrackEvent(name string, properties map[string]any) {
	if c.Analytics != nil {
		c.Analytics.Track(c.Context, name, properties)
	}
}

// SendEmailTemplate renders a file template and sends it using the default email provider.
func (c *ActionContext) SendEmailTemplate(to, templateName string, data any) error {
	if c.Comm == nil || c.Comm.EmailMgr == nil {
		return fmt.Errorf("email service not initialized")
	}
	subject, htmlBody, _, err := email.RenderFileTemplates(".", templateName, data)
	if err != nil {
		return err
	}
	return c.Comm.EmailMgr.Send(c.Context, "default", to, subject, htmlBody)
}

type ActionHandler func(*ActionContext, http.ResponseWriter, *http.Request)
