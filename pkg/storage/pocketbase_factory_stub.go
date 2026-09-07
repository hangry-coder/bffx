//go:build !experimental_pocketbase

package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
)

func newPocketBaseStoreFromConfig(*manifest.StoreConfig) (Store, error) {
	return nil, fmt.Errorf(`store mode "pocketbase" requires building with -tags=experimental_pocketbase (see docs/limitations.md)`)
}
