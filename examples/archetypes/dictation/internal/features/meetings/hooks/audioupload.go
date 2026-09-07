package hooks

import (
	"fmt"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

// BeforeAudioUpload executes custom preprocessing rules before the AudioUpload pipeline triggers.
// @bffx:action
func BeforeAudioUpload(ctx *handlers.ActionContext, payload map[string]any) error {
	fmt.Printf("beforePipeline executed for AudioUpload\n")
	return nil
}

// AfterAudioUpload executes custom postprocessing rules after the AudioUpload pipeline finishes.
// @bffx:action
func AfterAudioUpload(ctx *handlers.ActionContext, payload map[string]any) error {
	fmt.Printf("afterPipeline executed for AudioUpload\n")
	return nil
}
