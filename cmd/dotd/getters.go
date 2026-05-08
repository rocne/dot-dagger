package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// Hidden internal commands — not shown in default help.
// Intended for use in env.yaml shell expressions:
//
//	os: $(dotd get-os)
//	hostname: $(dotd get-hostname)

var getOSCmd = &cobra.Command{
	Use:    "get-os",
	Hidden: true,
	Short:  "Print normalized OS name (macos, linux, windows)",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := runtime.GOOS
		if name == "darwin" {
			name = "macos"
		}
		fmt.Fprintln(cmd.OutOrStdout(), strings.ToLower(name))
		return nil
	},
}

var getHostnameCmd = &cobra.Command{
	Use:    "get-hostname",
	Hidden: true,
	Short:  "Print system hostname",
	RunE: func(cmd *cobra.Command, args []string) error {
		h, err := os.Hostname()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), h)
		return nil
	},
}
