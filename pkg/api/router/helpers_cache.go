package router

import (
	corecache "github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
	"net/http"
	"strings"
	"time"
)

type tagCacheCapture struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (w *tagCacheCapture) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *tagCacheCapture) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// withTagCache wraps a handler with the tagged cache engine.
//
// Optional extraTags are appended to whatever spec.TagsFrom resolves to.
// This is how Screen and Section routes inject their `screen:<name>` and
// `screen:<name>:section:<key>` namespaces so the admin panel can flush
// just one feature's cache without touching unrelated entries.
func (r *Router) withTagCache(spec *manifest.CacheConfig, next http.HandlerFunc, extraTags ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if r.tagCache == nil || spec == nil || spec.TTL <= 0 {
			w.Header().Set("X-BFFX-Cache", "BYPASS")
			next(w, req)
			return
		}

		userID := middleware.GetUserID(req.Context())
		if userID == "" && !spec.AllowAnonymous {
			w.Header().Set("X-BFFX-Cache", "BYPASS")
			next(w, req)
			return
		}

		key := r.buildTagCacheKey(userID, req, spec.Vary)

		// 1. Try GET
		val, err := r.tagCache.Get(req.Context(), key)
		if err == nil && val != nil {
			etag := generateETag(val)
			w.Header().Set("Content-Type", "application/json")
			if etag != "" {
				w.Header().Set("ETag", etag)
				if req.Header.Get("If-None-Match") == etag {
					w.Header().Set("X-BFFX-Cache", "HIT")
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
			w.Header().Set("X-BFFX-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			w.Write(val)
			return
		}

		// 2. MISS path
		w.Header().Set("X-BFFX-Cache", "MISS")
		capture := &tagCacheCapture{ResponseWriter: w}
		next(capture, req)

		// 3. Store if successful
		if capture.status == http.StatusOK && len(capture.body) > 0 {
			tags := r.resolveTags(spec.TagsFrom, req)
			for _, t := range extraTags {
				if t != "" {
					tags = append(tags, t)
				}
			}
			_ = r.tagCache.SetWithTags(req.Context(), key, capture.body, tags, time.Duration(spec.TTL)*time.Second)
		}
	}
}

// ScreenCacheTag returns the canonical cache tag for a Screen.
func ScreenCacheTag(screen string) string {
	return corecache.ScreenTag(screen)
}

// SectionCacheTag returns the canonical cache tag for a Screen section.
func SectionCacheTag(screen, section string) string {
	return corecache.SectionTag(screen, section)
}

// InvalidateScreenCache flushes every cache entry tagged with the supplied
// Screen. Returns the number of entries invalidated (0 if the tag cache is
// not configured).
func (r *Router) InvalidateScreenCache(ctx context.Context, screen string) (int, error) {
	if r.tagCache == nil {
		return 0, nil
	}
	return r.tagCache.InvalidateTags(ctx, []string{ScreenCacheTag(screen)})
}

// InvalidateSectionCache flushes every cache entry tagged with the supplied
// Screen/Section pair.
func (r *Router) InvalidateSectionCache(ctx context.Context, screen, section string) (int, error) {
	if r.tagCache == nil {
		return 0, nil
	}
	return r.tagCache.InvalidateTags(ctx, []string{SectionCacheTag(screen, section)})
}

func (r *Router) buildTagCacheKey(userID string, req *http.Request, vary []string) string {
	return corecache.BuildHTTPScopedKey(corecache.TaggedResponseKeyPrefix, userID, req.Method, req.URL.Path, req.URL.Query(), vary, req.Header.Get)
}

func (r *Router) resolveTags(tagSources []string, req *http.Request) []string {
	tags := []string{}
	for _, src := range tagSources {
		// Example: "resource:User" -> literal "resource:User"
		// Example: "header:X-User-ID" -> "header:X-User-ID:{val}"
		if strings.HasPrefix(src, "header:") {
			hName := strings.TrimPrefix(src, "header:")
			if val := req.Header.Get(hName); val != "" {
				tags = append(tags, src+":"+val)
			}
		} else {
			tags = append(tags, src)
		}
	}
	return tags
}
