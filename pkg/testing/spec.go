// DEPRECATED: bffx/pkg/testing/spec is deprecated.
// Use standard go test with bffx/pkg/testing/audit for functional testing.
package testing

import (
	"fmt"
	"testing"
)

type Spec struct {
	t *testing.T
}

func NewSpec(t *testing.T) *Spec {
	return &Spec{t: t}
}

func (s *Spec) Describe(name string, f func()) {
	s.t.Run(name, func(t *testing.T) {
		f()
	})
}

func (s *Spec) Context(name string, f func()) {
	s.t.Run(name, func(t *testing.T) {
		f()
	})
}

func (s *Spec) It(name string, f func()) {
	s.t.Run(name, func(t *testing.T) {
		f()
	})
}

// Global helpers for cleaner DSL if using a shared T
var currentT *testing.T

func Describe(name string, f func()) {
	if currentT == nil {
		fmt.Printf("Describe: %s\n", name)
		f()
		return
	}
	currentT.Run(name, func(t *testing.T) {
		f()
	})
}

func Context(name string, f func()) {
	if currentT == nil {
		fmt.Printf("  Context: %s\n", name)
		f()
		return
	}
	currentT.Run(name, func(t *testing.T) {
		f()
	})
}

func It(name string, f func()) {
	if currentT == nil {
		fmt.Printf("    It: %s\n", name)
		f()
		return
	}
	currentT.Run(name, func(t *testing.T) {
		f()
	})
}

func SetT(t *testing.T) {
	currentT = t
}
