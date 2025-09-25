//go:build !metrics
// +build !metrics

package cli

import (
	"log/slog"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	zerr "zotregistry.dev/zot/errors"
	"zotregistry.dev/zot/pkg/exporter/api"
)

// metadataConfig reports metadata after parsing, which we use to track
// errors.
func metadataConfig(md *mapstructure.Metadata) viper.DecoderConfigOption {
	return func(c *mapstructure.DecoderConfig) {
		c.Metadata = md
	}
}

func NewExporterCmd() *cobra.Command {
	config := api.DefaultConfig()

	// "config"
	configCmd := &cobra.Command{
		Use:     "config <config_file>",
		Aliases: []string{"config"},
		Short:   "`config` node exporter properties",
		Long:    "`config` node exporter properties",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				loadConfiguration(config, args[0])
			}

			c := api.NewController(config)
			c.Run()
		},
	}

	// "node_exporter"
	exporterCmd := &cobra.Command{
		Use:   "zxp",
		Short: "`zxp`",
		Long:  "`zxp`",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Usage()
			cmd.SilenceErrors = false
		},
	}

	exporterCmd.AddCommand(configCmd)

	return exporterCmd
}

func loadConfiguration(config *api.Config, configPath string) {
	viper.SetConfigFile(configPath)

	if err := viper.ReadInConfig(); err != nil {
		slog.Error("failed to read configuration", "error", err, "config", configPath)
		panic(err)
	}

	metaData := &mapstructure.Metadata{}
	if err := viper.Unmarshal(&config, metadataConfig(metaData)); err != nil {
		slog.Error("failed to unmarshal config", "error", err, "config", configPath)
		panic(err)
	}

	if len(metaData.Keys) == 0 {
		slog.Error("bad configuration", "error", zerr.ErrBadConfig, "config", configPath)
		panic(zerr.ErrBadConfig)
	}

	if len(metaData.Unused) > 0 {
		slog.Error("bad configuration", "error", zerr.ErrBadConfig, "config", configPath, 
			"unknown fields", metaData.Unused)
		panic(zerr.ErrBadConfig)
	}
}
