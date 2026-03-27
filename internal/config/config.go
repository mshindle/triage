package config

import (
	"fmt"
	"net/url"

	"github.com/go-playground/validator"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Config struct {
	Signal struct {
		URL string `mapstructure:"url" validate:"required,url"`
	} `mapstructure:"signal"`

	Database struct {
		URL string `mapstructure:"url" validate:"required,url"`
	} `mapstructure:"database"`

	LLM struct {
		Key        string `mapstructure:"key" validate:"required,min=10"`
		Model      string `mapstructure:"model" validate:"required"`
		EmbedModel string `mapstructure:"embed_model" validate:"required"`
		EmbedDims  int    `mapstructure:"embed_dims" validate:"required,gte=512"`
	} `mapstructure:"llm"`

	Log struct {
		Level   string `mapstructure:"level" validate:"oneof=debug info warn error"`
		Console bool   `mapstructure:"console"`
	} `mapstructure:"log"`

	Web struct {
		ListenAddr string `mapstructure:"listen_addr" validate:"required"`
	} `mapstructure:"web"`
}

func (c Config) MarshalZerologObject(e *zerolog.Event) {
	u, _ := url.Parse(c.Database.URL)
	if u.User != nil {
		username := u.User.Username()
		u.User = url.UserPassword(username, "****")
	}

	e.Dict("signal", zerolog.Dict().Str("url", c.Signal.URL))
	e.Dict("database", zerolog.Dict().Str("url", u.String()))
	e.Dict("llm", zerolog.Dict().Str("key", fmt.Sprintf("%s...", c.LLM.Key[:5])))
	e.Dict("log", zerolog.Dict().Str("level", c.Log.Level).Bool("console", c.Log.Console))
}

func LoadConfig(v *viper.Viper) (*Config, error) {
	var config Config

	// 5. Final Step: Unmarshal into the struct
	// Viper will prioritize: Env Var > .env File > Defaults
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 6. Validation Step
	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}
