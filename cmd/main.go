package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.zouai.io/colossus/clog"
)

var rootCtx context.Context
var logger *clog.Logger

func main() {
	rootCtx = context.Background()
	rootCtx, logger = clog.NewRootLogger(rootCtx, "KubeSecretPipe")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "kube-secret-pipe",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var initMethods []func()

func init() {
	for _, i := range initMethods {
		i()
	}
}

func onInit(f func()) bool {
	initMethods = append(initMethods, f)
	return true
}

func hasKubernetesServiceAccount() bool {
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount")
	if err != nil {
		return false
	}
	return true
}
