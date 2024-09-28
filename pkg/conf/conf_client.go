//go:build !xuandb_server

package conf

func init() {
}

// LoadClient loads client configurations.
func LoadClient() error {
	return load()
}
