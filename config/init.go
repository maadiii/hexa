package config

func init() {
	database = new(databaseCfg).init()
}
