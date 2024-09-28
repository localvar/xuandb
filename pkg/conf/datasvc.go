package conf

// DataServiceConf contains configuration for the data service.
type DataServiceConf struct {
	DataDir string `toml:"data-dir" json:"dataDir"`
}

// defaultDataServiceConf contains the default values for DataServiceConf.
var defaultDataServiceConf = &DataServiceConf{}

// fillDefaults fills the default values for DataServiceConf.
func (dsc *DataServiceConf) fillDefaults(dflt *DataServiceConf) {
	if dsc.DataDir == "" {
		dsc.DataDir = dflt.DataDir
	}
}

// normalizeAndValidate normalizes & validates the DataServiceConf.
func (dsc *DataServiceConf) normalizeAndValidate() error {
	return nil
}
