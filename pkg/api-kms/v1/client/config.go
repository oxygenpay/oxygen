package client

type Config struct {
	Host     string   `yaml:"host" env:"KMS_CLIENT_HOST" env-default:"localhost:14000" env-description:"KMS server host. Example: localhost:14000"`
	BasePath string   `yaml:"base_path" env:"KMS_CLIENT_BASE_PATH" env-default:"/api/kms/v1" env-description:"Internal variable"`
	Schemes  []string `yaml:"schemes" env:"KMS_CLIENT_SCHEMES" env-default:"http" env-description:"Internal variable"`
}
