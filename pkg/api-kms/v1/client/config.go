package client

type Config struct {
	Host     string   `yaml:"host" env:"KMS_CLIENT_HOST"`
	BasePath string   `yaml:"base_path" env:"KMS_CLIENT_BASE_PATH"`
	Schemes  []string `yaml:"schemes" env:"KMS_CLIENT_SCHEMES"`
}
