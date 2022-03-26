package core

type Config struct {
	Hosts map[string]ConfigHost `yaml:"hosts"`
}

type ConfigHost struct {
	Name string `yaml:"name"`

	Addr string `yaml:"addr"`
	User string `yaml:"user"`
	// TODO: some jumphost config
}
