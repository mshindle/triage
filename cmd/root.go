package cmd

import (
	"os"
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

	_ = v.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log_level"))
	_ = v.BindPFlag("log_console", rootCmd.PersistentFlags().Lookup("console"))
}

func initConfig() {
	viper.SetEnvPrefix("TRIAGE")
	viper.AutomaticEnv() // e.g. TRIAGE_SIGNAL_URL
	viper.SetDefault("db_url", "postgres://localhost:5432/triage?sslmode=disable")
	viper.SetDefault("signal_url", "ws://localhost:8080/v1/receive/+1234567")
}

func globalPreRun(_ *cobra.Command, _ []string) error {
	return configLogger()
}

func configLogger() error {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	lvl, err := zerolog.ParseLevel(v.GetString("log_level"))
	if err != nil {
		return err
	}
	zlog = zlog.Level(lvl)

	if v.GetBool("log_console") {
		zlog = zlog.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Set the global logger to our configured logger
	log.Logger = zlog
	return nil
}
