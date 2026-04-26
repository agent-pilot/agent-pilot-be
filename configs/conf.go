package configs

import (
	"strings"

	"github.com/spf13/viper"
)

// Conf is the application configuration loaded from `configs/config.yaml`
// and/or environment variables (via Viper).
type Conf struct {
	OpenAI OpenAIConfig `mapstructure:"openai"`
}

// OpenAIConfig matches `configs/config.yaml` keys.
// Note: for Volc Ark, `endpointID` is used as the "model" / endpoint identifier.
type OpenAIConfig struct {
	APIKey     string `mapstructure:"apiKey"`
	EndpointID string `mapstructure:"endpointID"`
	BaseURL    string `mapstructure:"baseUrl"`
}

func Load(path string) (*Conf, error) {

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(
		strings.NewReplacer(".", "_"),
	)
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var c Conf
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil

}
