package main

import (
	v1 "github.com/fatedier/frp/pkg/config/v1"

	"github.com/spf13/cobra"
)

var (
	cfgFile          string
	showVersion      bool
	strictConfigMode bool
	serverCfg        v1.ServerConfig
)

func init() {
	rootCmd.PersistentFlags().String
}

// 用cobra来实现客户端命令行能力
var rootCmd = &cobra.Command{
	Use:   "frps",
	Short: "frps is the server of frp",
	RunE: func(cmd *cobra.Command, args []string) {
	},
}
