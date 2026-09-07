package cli

import (
	"fmt"
	"os"
	"os/exec"
)

func HandleTest(args []string) {
	fmt.Println("🚀 Running BFFX Spec Suite...")

	// 1. Run User Specs
	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("\n❌ Tests failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✅ All specs passed!")
}
