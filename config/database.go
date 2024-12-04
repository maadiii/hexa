package config

func Database() *databaseCfg { //nolint
	return database
}

var database *databaseCfg

type databaseCfg struct {
	Name          string
	Username      string
	Password      string
	Schema        string
	Driver        string
	Host          string
	Port          string
	MigrationPath string
}

func (d *databaseCfg) init() *databaseCfg {
	return d
}
