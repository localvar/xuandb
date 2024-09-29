// 'xuandb_editor' is to satisfy code editors, not used in the build process.
// e.g. in vscode, add the following to settings.json to make the editor happy.
//        "gopls": {
//                "build.buildFlags": ["-tags=xuandb_editor"]
//        }
//
//go:build !xuandb_server || xuandb_editor

package conf

func init() {
}

// LoadClient loads client configurations.
func LoadClient() error {
	return load()
}
