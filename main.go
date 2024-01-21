package main

import (
	"io"
	"runtime/debug"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	Version      string
	CommitSHA    string
	quietFlag    bool
	parallelFlag int

	rootCmd = &cobra.Command{
		Use:           "gyz [<file|dir>]",
		Short:         "Run a given files and upload its to Gyazo.",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if quietFlag {
				log.SetOutput(io.Discard)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// NOTE: デフォルトをアップロードにする。この呼び出し方だと、 `upload` 側のPre/PostRunが呼び出されないので注意
			return uploadCommandHandler(cmd, args, parallelFlag)
		},
	}

	uploadCmd = &cobra.Command{
		Use:   "upload",
		Short: "Create a new tape file by recording your actions",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if quietFlag {
				log.SetOutput(io.Discard)
			}
		},
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return uploadCommandHandler(cmd, args, parallelFlag)
		},
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "quiet do not log messages")
	rootCmd.Flags().IntVarP(&parallelFlag, "parallel", "p", 5, "number of parallel uploads")
	uploadCmd.Flags().IntVarP(&parallelFlag, "parallel", "p", 5, "number of parallel uploads")
	rootCmd.AddCommand(
		uploadCmd,
	)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	if len(CommitSHA) >= 7 {
		vt := rootCmd.VersionTemplate()
		rootCmd.SetVersionTemplate(vt[:len(vt)-1] + " (" + CommitSHA[0:7] + ")\n")
	}
	if Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			Version = info.Main.Version
		} else {
			Version = "unknown (built from source)"
		}
	}
	rootCmd.Version = Version
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
	}
}
