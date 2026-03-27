package cmd

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var v = viper.New()
var zlog = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.InfoLevel)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "triage",
	Short: "Intelligent AI triage engine for Signal messages",
	Long: `The Signal AI Triage Engine acts as an intelligent middleware for Signal messages,
providing real-time priority scoring, automated categorization, and a semantic
feedback loop to help operators manage their message flow more effectively.

Key features include:
- Real-time ingestion and display of Signal messages.
- AI-driven priority assignment (0-100) and reasoning.
- Interactive feedback to train the AI's classification.
- Direct message replying from the dashboard.`,
	PersistentPreRunE: globalPreRun,
	SilenceErrors:     true,
	SilenceUsage:      true,
	Version:           "0.0.1",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {

		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().String("log_level", "INFO", "sets logging level")
	rootCmd.PersistentFlags().Bool("console", false, "formats logs for console reading")
	rootCmd.PersistentFlags().String("llm_key", "", "setting via env var - TRIAGE_LLM_KEY - is preferred")

	_ = v.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log_level"))
	_ = v.BindPFlag("log.console", rootCmd.PersistentFlags().Lookup("console"))
}

func initConfig() {
	// 1. Set Defaults
	// This "teaches" Viper that these keys exist so Unmarshal doesn't skip them
	v.SetDefault("db.url", "postgres://localhost:5432/triage?sslmode=disable")
	v.SetDefault("signal.url", "ws://localhost:8080/v1/receive/+1234567")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.console", false)
	v.SetDefault("llm.key", "supply-a-key")
	v.SetDefault("llm.model", "gpt-4o-mini")
	v.SetDefault("llm.embed_model", "text-embedding-3-small")
	v.SetDefault("llm.embed_dims", 768)
	v.SetDefault("web.listen_addr", ":8081")

	// 2. Setup Environment Variable Logic
	v.SetEnvPrefix("TRIAGE") // Prepends "TRIAGE_" to all env lookups
	v.AutomaticEnv()         // Tell Viper to look for matching env vars

	// This is critical: It allows "signal_url" to match "TRIAGE_SIGNAL_URL"
	// and handles nested structures if you use dots like "db.host" -> "TRIAGE_DB_HOST"
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 3. Set up File Reading (.env)
	v.AddConfigPath(".")
	v.SetConfigName("triage")
	v.SetConfigType("yaml")

	// 4. Read the file
	if err := v.ReadInConfig(); err != nil {
		_, ok := errors.AsType[viper.ConfigFileNotFoundError](err)
		if !ok {
			log.Error().Err(err).Msg("failed to read config")
			return
		}
		log.Info().Msg("no .env file found; proceeding with environment variables and defaults")
	}
}

func globalPreRun(_ *cobra.Command, _ []string) error {
	return configLogger()
}

func configLogger() error {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	lvl, err := zerolog.ParseLevel(v.GetString("log.level"))
	if err != nil {
		return err
	}
	zlog = zlog.Level(lvl)

	if v.GetBool("log.console") {
		zlog = zlog.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Set the global logger to our configured logger
	log.Logger = zlog
	return nil
}
