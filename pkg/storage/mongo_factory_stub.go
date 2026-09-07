//go:build !experimental_mongo

package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
)

func newMongoStoreFromConfig(*manifest.StoreConfig) (Store, error) {
	return nil, fmt.Errorf(`store mode "mongo" requires building with -tags=experimental_mongo (see docs/limitations.md)`)
}
