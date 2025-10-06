package schema

type Config struct {
	Services map[string]Service `yaml:"services" json:"services"`
}
