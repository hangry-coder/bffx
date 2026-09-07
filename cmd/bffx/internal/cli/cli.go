package cli

import (
	"fmt"
)

// Version is set by main for usage/help text.
var Version string

func Usage() {
	fmt.Printf("bffx %s\n", Version)
	fmt.Println("usage:")
	fmt.Println("  bffx new NAME")
	fmt.Println("  bffx sync [--dry-run]")
	fmt.Println("  bffx dev")
	fmt.Println("  bffx build")
	fmt.Println("  bffx test")
	fmt.Println("  bffx routes list")
	fmt.Println("  bffx generate resource NAME field:type ...")
	fmt.Println("  bffx generate scaffold NAME field:type ...")
	fmt.Println("  bffx generate builder NAME")
	fmt.Println("  bffx generate action NAME")
	fmt.Println("  bffx generate skill NAME")
	fmt.Println("  bffx generate function NAME")
	fmt.Println("  bffx generate service NAME")
	fmt.Println("  bffx generate stream NAME")
	fmt.Println("  bffx generate pipeline TYPE --name NAME --feature FEATURE [--catalog=ADAPTER]")
	fmt.Println("  bffx generate client [platform]")
	fmt.Println("  bffx add admin")
	fmt.Println("  bffx env check")
	fmt.Println("  bffx mcp serve [--transport stdio|http]")
	fmt.Println("  bffx doctor")
	fmt.Println("  bffx migrate [init|plan|apply|status|diff|layout|packaging]")
	fmt.Println("  bffx db backup [--path FILE]   (SQLite-only, online VACUUM INTO snapshot)")
	fmt.Println("  bffx upgrade")
	fmt.Println("  bffx deploy [init|ship|rollback|status|cloud]")
	fmt.Println("  bffx update framework [--vendor-only] [--root DIR] [--dry-run] [--force]")
	fmt.Println("  bffx lint [--root DIR] [--strict]")
	fmt.Println("  bffx seed [--root DIR] [--upsert]")
	fmt.Println("  bffx check [--root DIR] [--strict]")
}
