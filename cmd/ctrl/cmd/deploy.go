/*
Sniperkit-Bot
- Status: analyzed
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"we.com/dolphin/types"
	"we.com/jiabiao/common/yaml"
)

var (
	env       string
	deployKey string
)

func init() {
	cmdDeploy.AddCommand(deployAdd)
	cmdDeploy.AddCommand(deployDelete)
	cmdDeploy.AddCommand(deployEdit)
	cmdDeploy.AddCommand(deployList)
}

var cmdDeploy = &cobra.Command{
	Use:   "deploy",
	Short: "manipulate deploy specific infos",
	Long:  `manipulate deploy specific infos`,
	Args:  cobra.MinimumNArgs(1),
	Run:   nil,
}

var deployAdd = &cobra.Command{
	Use:   "add <cfg.yml>",
	Short: "add a new deploy if it is not already exists",
	Long:  `add a new deploy if it is not already exists`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dc := types.DeployConfig{}

		for _, v := range args {
			fmt.Printf("args: %v", v)
		}

		file, err := os.Open(args[1])
		if err != nil {
			glog.Errorf("open %v err: %v", args[1], err)
			return
		}
		decode := yaml.NewYAMLOrJSONDecoder(file, 4)

		if err = decode.Decode(&dc); err != nil {
			glog.Errorf("decode config file err: %v", err)
			return
		}

		if err := dc.Validate(); err != nil {
			glog.Errorf("deploy config is not valid: %v", err)
			return
		}

	},
}

var deployEdit = &cobra.Command{
	Use:   "edit <env> <deployKey>",
	Short: "edit an exist deployment",
	Long:  `edit an exist deployment`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var deployDelete = &cobra.Command{
	Use:   "rm <env> <deployKey>",
	Short: "remove an exist deployment",
	Long:  `remove an exist deployment`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var deployList = &cobra.Command{
	Use:   "list ",
	Short: "list current deployments",
	Long:  "list current deployments",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

	},
}
