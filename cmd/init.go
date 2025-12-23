package cmd

import (
	"fmt"

	"github.com/ptone/scion/pkg/config"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize scion in the current project",
	Long: `Initialize scion by creating the .scion directory structure
and seeding the default template. It also ensures the global ~/.scion
directory exists for playground groves.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Initializing scion project...")
		if err := config.InitProject(); err != nil {
			return fmt.Errorf("failed to initialize project: %w", err)
		}

		fmt.Println("Initializing global scion directory...")
		if err := config.InitGlobal(); err != nil {
			return fmt.Errorf("failed to initialize global config: %w", err)
		}

		fmt.Println("scion successfully initialized.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
