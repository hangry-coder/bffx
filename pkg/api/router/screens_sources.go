package router

import (
	"context"
	"net/http"
	"sort"
	"sync"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"

	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
)

func (r *Router) resolveSources(req *http.Request, sources []any, partialSuccess bool) (map[string]any, []error) {
	sourceData := make(map[string]any)
	var mu sync.Mutex
	var errs []error
	var errMu sync.Mutex

	ctx, span := observability.StartSpan(req.Context(), "ResolveSources")
	defer span.End()

	g, ctx := errgroup.WithContext(ctx)

	for _, src := range sources {
		src := src // capture for closure
		g.Go(func() error {
			srcName := "unknown"
			if s, ok := src.(string); ok {
				srcName = s
			} else if mSrc, ok := src.(map[string]any); ok {
				kind, _ := mSrc["kind"].(string)
				name, _ := mSrc["name"].(string)
				if kind != "" && name != "" {
					srcName = kind + ":" + name
				} else if name != "" {
					srcName = name
				}
			}

			subCtx, subSpan := observability.StartSpan(ctx, "ResolveSource: "+srcName)
			defer subSpan.End()
			subSpan.SetAttributes(attribute.String("source.name", srcName))

			res, err := r.resolveSingleSource(subCtx, req, src)
			if err != nil {
				subSpan.RecordError(err)
				if partialSuccess {
					errMu.Lock()
					errs = append(errs, err)
					errMu.Unlock()
					return nil // Don't abort others
				}
				return err
			}

			mu.Lock()
			for k, v := range res {
				sourceData[k] = v
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		span.RecordError(err)
		return nil, []error{err}
	}

	return sourceData, errs
}

func (r *Router) resolveSingleSource(ctx context.Context, req *http.Request, src any) (map[string]any, error) {
	res := make(map[string]any)
	if s, ok := src.(string); ok {
		switch s {
		case "app":
			proj := r.reg.ProjectSpec()
			locale := middleware.GetLocale(ctx)
			permissions := make([]map[string]any, 0, len(proj.App.Permissions))
			for _, p := range proj.App.Permissions {
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
				permissions = append(permissions, map[string]any{
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

			res["app"] = map[string]any{
				"name":             r.reg.Project.Metadata.Name,
				"version":          "0.1.0",
				"apiPrefix":        proj.App.ApiPrefix,
				"minClientVersion": proj.App.MinClientVersion,
				"permissions":      permissions,
				"ui":               proj.App.UI,
				"menu":             proj.App.Menu,
			}
		case "currentUser":
			userID := middleware.GetUserID(ctx)
			if userID != "" {
				user, err := r.store.Get(ctx, "User", userID)
				if err != nil {
					return nil, err
				}
				res["currentUser"] = user
			} else {
				res["currentUser"] = nil
			}
		case "i18n":
			locale := middleware.GetLocale(ctx)
			overrides := make(map[string]string)
			if r.store != nil {
				items, err := r.store.Query(ctx, "AppString").Where("locale", "=", locale).Execute(ctx)
				if err == nil {
					for _, item := range items {
						k, _ := item["key"].(string)
						v, _ := item["value"].(string)
						if k != "" {
							overrides[k] = v
						}
					}
				}
			}
			res["i18n"] = r.i18n.GetMap(locale, overrides)
		case "navigation":
			var bottomNav []map[string]any
			var foldedMenu []map[string]any

			sort.Slice(r.reg.Screens, func(i, j int) bool {
				var specI, specJ manifest.ScreenSpec
				r.reg.Screens[i].UnmarshalSpec(&specI)
				r.reg.Screens[j].UnmarshalSpec(&specJ)
				return specI.Order < specJ.Order
			})

			for _, m := range r.reg.Screens {
				var sSpec manifest.ScreenSpec
				m.UnmarshalSpec(&sSpec)
				item := map[string]any{
					"name":          sSpec.Name,
					"icon":          sSpec.Icon,
					"requires_auth": sSpec.RequiresAuth,
				}
				if sSpec.NavType == "bottom" {
					bottomNav = append(bottomNav, item)
				} else if sSpec.NavType == "folded" {
					foldedMenu = append(foldedMenu, item)
				}
			}
			res["navigation"] = map[string]any{
				"bottom_nav":  bottomNav,
				"folded_menu": foldedMenu,
			}
		case "onboarding":
			proj := r.reg.ProjectSpec()
			authStrategy := proj.App.AuthStrategy
			if authStrategy == "" {
				authStrategy = "optional"
			}
			onboardingSequence := []string{"intro_carousel", "permission_request", "home"}
			if authStrategy == "optional" || authStrategy == "mandatory" {
				onboardingSequence = []string{"intro_carousel", "auth_gateway", "personalization_step", "home"}
			}
			res["onboarding"] = map[string]any{
				"onboarding_sequence": onboardingSequence,
				"can_skip":            authStrategy == "optional",
			}
		case "flags":
			userID := middleware.GetUserID(ctx)
			deviceID := req.Header.Get("X-Device-ID")
			claims := middleware.GetClaims(ctx)
			fctx := featureflags.EvalContext{
				UserID:   userID,
				DeviceID: deviceID,
				Claims:   claims,
			}
			res["flags"] = r.featureFlags.GetAllResolved(fctx)
		}
	} else if mSrc, ok := src.(map[string]any); ok {
		kind, _ := mSrc["kind"].(string)
		name, _ := mSrc["name"].(string)
		if kind == "Resource" && name != "" {
			items, err := r.store.List(ctx, name, 0, 0)
			if err != nil {
				return nil, err
			}
			res[name] = items
		}
	}
	return res, nil
}

func (r *Router) resolveOutput(ctx context.Context, sourceData map[string]any, outputSpec map[string]any) map[string]any {
	_, span := observability.StartSpan(ctx, "ResolveOutput")
	defer span.End()

	output := make(map[string]any)
	for outKey, srcPath := range outputSpec {
		if sPath, ok := srcPath.(string); ok {
			val := resolvePath(sourceData, sPath)
			setNested(output, outKey, val)
		}
	}
	return output
}
