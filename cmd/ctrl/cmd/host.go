package cmd

import "github.com/spf13/cobra"

var cmdHost = &cobra.Command{
	Use:   "host",
	Short: "manipulate deploy specific infos",
	Long:  `manipulate deploy specific infos`,
	Args:  cobra.MinimumNArgs(1),
	Run:   nil,
}
