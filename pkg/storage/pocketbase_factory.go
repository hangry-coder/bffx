//go:build experimental_pocketbase

package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
)

func newPocketBaseStoreFromConfig(conf *manifest.StoreConfig) (Store, error) {
	if conf.Url == "" {
		return nil, fmt.Errorf("pocketbase url is required")
	}
	return NewPocketBaseStore(conf.Url, conf.ApiKey), nil
}
