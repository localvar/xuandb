package conf

// QueryServiceConf contains configuration for the query service.
type QueryServiceConf struct {
}

// defaultQueryServiceConf contains the default values for QueryServiceConf.
var defaultQueryServiceConf = &QueryServiceConf{}

// fillDefaults fills the default values for QueryServiceConf.
func (qsc *QueryServiceConf) fillDefaults(dflt *QueryServiceConf) {
}

// normalizeAndValidate normalizes & validates the QueryServiceConf.
func (qsc *QueryServiceConf) normalizeAndValidate() error {
	return nil
}
