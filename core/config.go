package core

type Config struct {
	Hosts map[string]ConfigHost `yaml:"hosts"`
}

type ConfigHost struct {
	Name string `yaml:"name"`

	Hostname string `yaml:"hostname"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	// TODO: some jumphost config
}
