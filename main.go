package main

import (
	"io"
	"os"
	"runtime/debug"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/upamune/gyz/internal/gyazo"
)

var (
	Version   string
	CommitSHA string
	quietFlag bool

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
			return uploadCommandHandler(cmd, args)
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
			return uploadCommandHandler(cmd, args)
		},
	}
)

func setFlags() {
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "quiet do not log messages")

	// NOTE: rootCmdをuploadCmdと同じ挙動にしたいので両方に登録している。他のサブコマンドでは利用したくないのでLocal Flagsにしている
	rootCmd.Flags().IntP("parallel", "p", 5, "number of parallel uploads")
	uploadCmd.Flags().IntP("parallel", "p", 5, "number of parallel uploads")

	rootCmd.Flags().BoolP("interactive", "i", false, "interactive mode")
	uploadCmd.Flags().BoolP("interactive", "i", false, "interactive mode")

	rootCmd.Flags().String("desc", "", "description")
	uploadCmd.Flags().String("desc", "", "description")

	rootCmd.Flags().String("app", "", "app")
	uploadCmd.Flags().String("app", "", "app")

	rootCmd.Flags().String("access-policy", "", "access policy")
	uploadCmd.Flags().String("access-policy", "", "access policy")

	rootCmd.Flags().Bool("metadata-is-public", false, "metadata is public")
	uploadCmd.Flags().Bool("metadata-is-public", false, "metadata is public")

	rootCmd.Flags().Bool("exif", false, "using exif")
	uploadCmd.Flags().Bool("exif", false, "using exif")
}

func init() {
	setFlags()
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
	os.Exit(realMain())
}

func realMain() int {
	if os.Getenv(gyazo.AccessTokenEnvName) == "" {
		log.Errorf("environment variable %s is not set", gyazo.AccessTokenEnvName)
		return 1
	}
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		return 1
	}
	return 0
}
