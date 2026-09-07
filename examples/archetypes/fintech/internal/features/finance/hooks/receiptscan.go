package hooks

import (
	"fmt"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

// BeforeReceiptScan executes custom preprocessing rules before the ReceiptScan pipeline triggers.
// @bffx:action
func BeforeReceiptScan(ctx *handlers.ActionContext, payload map[string]any) error {
	fmt.Printf("beforePipeline executed for ReceiptScan\n")
	return nil
}

// AfterReceiptScan executes custom postprocessing rules after the ReceiptScan pipeline finishes.
// @bffx:action
func AfterReceiptScan(ctx *handlers.ActionContext, payload map[string]any) error {
	fmt.Printf("afterPipeline executed for ReceiptScan\n")
	return nil
}
