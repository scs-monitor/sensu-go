// +build windows

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sensu/sensu-go/util/logging"
	"github.com/sensu/sensu-go/util/path"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	serviceName        = "SensuAgent"
	serviceDisplayName = "Sensu Agent"
	serviceDescription = "The monitoring agent for sensu-go (https://sensu.io)"
	serviceUser        = "LocalSystem"

	flagLogPath              = "log-file"
	flagLogMaxSize           = "log-max-size"
	flagLogRetentionDuration = "log-retention-duration"
	flagLogRetentionFiles    = "log-retention-files"
)

// NewWindowsServiceCommand creates a cobra command that offers subcommands
// for installing, uninstalling and running sensu-agent as a windows service.
func NewWindowsServiceCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "service",
		Short: "operate sensu-agent as a windows service",
	}

	command.AddCommand(NewWindowsInstallServiceCommand())
	command.AddCommand(NewWindowsUninstallServiceCommand())
	command.AddCommand(NewWindowsRunServiceCommand())

	return command
}

// NewWindowsInstallServiceCommand creates a cobra command that installs a
// sensu-agent service in Windows.
func NewWindowsInstallServiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "install",
		Short:         "install the sensu-agent service",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := cmd.Flag(flagConfigFile).Value.String()
			p, err := filepath.Abs(configFile)
			if err != nil {
				return fmt.Errorf("error reading config file: %s", err)
			}
			fi, err := os.Stat(p)
			if err != nil {
				return fmt.Errorf("error reading config file: %s", err)
			}
			if !fi.Mode().IsRegular() {
				return errors.New("error reading config file: not a regular file")
			}

			logPath := viper.GetString(flagLogPath)
			maxSize := viper.GetSizeInBytes(flagLogMaxSize)
			if maxSize == 0 {
				return fmt.Errorf("invalid max size: %s", viper.GetString(flagLogMaxSize))
			}
			retentionDuration := viper.GetDuration(flagLogRetentionDuration)
			retentionFiles := viper.GetInt64(flagLogRetentionFiles)
			cfg := logging.RotateFileLoggerConfig{
				Path:              logPath,
				MaxSizeBytes:      int64(maxSize),
				RetentionDuration: retentionDuration,
				RetentionFiles:    retentionFiles,
			}
			logWriter, err := logging.NewRotateFileLogger(cfg)
			if err != nil {
				return fmt.Errorf("error reading log file: %s", err)
			}

			return installService(serviceName, serviceDisplayName, serviceDescription, "service", "run", configFile, logWriter)
		},
	}

	defaultConfigPath := fmt.Sprintf("%s\\agent.yml", path.SystemConfigDir())
	defaultLogPath := fmt.Sprintf("%s\\sensu-agent.log", path.SystemLogDir())

	cmd.Flags().StringP(flagConfigFile, "c", defaultConfigPath, "path to sensu-agent config file")
	cmd.Flags().StringP(flagLogPath, "", defaultLogPath, "path to the sensu-agent log file")
	cmd.Flags().StringP(flagLogMaxSize, "", "128 MB", "maximum size of log file")
	cmd.Flags().StringP(flagLogRetentionDuration, "", "168h", "log file retention duration (s, m, h)")
	cmd.Flags().Int64P(flagLogRetentionFiles, "", 10, "maximum number of archived files to retain")

	return cmd
}

// NewWindowsUninstallServiceCommand creates a cobra command that uninstalls a
// sensu-agent service in Windows.
func NewWindowsUninstallServiceCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "uninstall",
		Short:         "uninstall the sensu-agent service",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeService(serviceName)
		},
	}
}

func NewWindowsRunServiceCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "run",
		Short:         "run the sensu-agent service (blocking)",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runService(args)
		},
	}
	return command
}
