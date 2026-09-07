package sse

import (
	"net/http"
	"reflect"
)

// unwrapFlusher walks nested ResponseWriter wrappers to find an http.Flusher.
func unwrapFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	if flusher, ok := w.(http.Flusher); ok {
		return flusher, true
	}

	v := reflect.ValueOf(w)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanInterface() {
				if parent, ok := field.Interface().(http.ResponseWriter); ok {
					if flusher, ok := unwrapFlusher(parent); ok {
						return flusher, true
					}
				}
			}
		}
	}
	return nil, false
}
