//go:build experimental_mongo

package storage

import "github.com/hangry-coder/bffx/pkg/manifest"

func newMongoStoreFromConfig(conf *manifest.StoreConfig) (Store, error) {
	url := conf.Url
	if url == "" {
		url = "mongodb://localhost:27017"
	}
	return NewMongoStore(url, "bffx_telemetry")
}
