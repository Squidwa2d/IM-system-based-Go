package util

import (
	"github.com/spf13/viper"
	"time"
)

type Config struct {
	DBSource             string        `mapstructure:"DB_SOURCE"`
	TokenSymmetricKey    string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	ServerAddress        string        `mapstructure:"SERVER_ADDRESS"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
	RdbSource            string        `mapstructure:"RDB_SOURCE"`
	MinioBucketName      string        `mapstructure:"MINIO_BUCKET_NAME"`
	MinioEndpoint        string        `mapstructure:"MINIO_ENDPOINT"`
	MinioAccessKey       string        `mapstructure:"MINIO_ACCESS_KEY"`
	MinioSecretKey       string        `mapstructure:"MINIO_SECRET_KEY"`
	MinioUseSSL          bool          `mapstructure:"MINIO_USE_SSL"`
}

// LoadConfig reads configuration from .env file
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
