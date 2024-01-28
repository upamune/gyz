package main

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/cockroachdb/errors"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/upamune/gyz/internal/gyazo"
)

func buildGyazoUploadOptionWithFlags(flags *flag.FlagSet) (gyazo.UploadOption, error) {
	desc, err := flags.GetString("desc")
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}
	app, err := flags.GetString("app")
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}
	accessPolicy, err := flags.GetString("access-policy")
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}
	metadataIsPublic, err := flags.GetBool("metadata-is-public")
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}

	return gyazo.UploadOption{
		App:              app,
		Desc:             desc,
		AccessPolicy:     accessPolicy,
		MetadataIsPublic: metadataIsPublic,
	}, nil

}

func buildGyazoUploadOptionWithInteractive() (gyazo.UploadOption, error) {
	var (
		accessPolicy     = "anyone"
		app              = "gyz"
		metadataIsPublic bool
		desc             string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AccessPolicy").
				Description("画像の公開範囲").
				Options(huh.NewOptions("anyone", "only_me")...).
				Value(&accessPolicy),
			huh.NewSelect[bool]().
				Title("MetadataIsPublic").
				Description("URLやタイトルなどのメタデータを公開するか否か").
				Options(huh.NewOptions(true, false)...).
				Value(&metadataIsPublic),
			huh.NewInput().Title("App").Description("キャプチャをしたアプリケーション名").Value(&app),
			huh.NewInput().Title("Description").Description("任意のコメント").Value(&desc),
		),
	)

	if err := form.Run(); err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}

	return gyazo.UploadOption{
		AccessPolicy:     accessPolicy,
		MetadataIsPublic: metadataIsPublic,
		App:              app,
		Desc:             desc,
	}, nil
}

func buildGyazoUploadOption(flags *flag.FlagSet) (gyazo.UploadOption, error) {
	interactive, err := flags.GetBool("interactive")
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}

	if interactive {
		opt, err := buildGyazoUploadOptionWithInteractive()
		if err != nil {
			return gyazo.UploadOption{}, errors.WithStack(err)
		}
		return opt, nil
	}

	opt, err := buildGyazoUploadOptionWithFlags(flags)
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}
	return opt, nil
}

func uploadCommandHandler(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()

	parallel, err := flags.GetInt("parallel")
	if err != nil {
		return errors.WithStack(err)
	}

	option, err := buildGyazoUploadOption(flags)
	if err != nil {
		return errors.WithStack(err)
	}

	p := pool.New().WithErrors().WithContext(cmd.Context()).WithMaxGoroutines(parallel)
	for _, arg := range args {
		arg := arg
		p.Go(func(ctx context.Context) error {
			info, err := os.Stat(arg)
			if err != nil {
				log.Warn("skip file because of stat error", "err", err, "filepath", arg)
				return nil
			}

			if info.IsDir() {
				return errors.WithStack(scandir(arg, option))
			}

			return errors.WithStack(scanfile(arg, info.Name(), option))
		})
	}
	return errors.WithStack(p.Wait())
}

func isSupportedImageFile(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg", ".png", ".gif":
		return true
	default:
		return false
	}
}

func scandir(dir string, option gyazo.UploadOption) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		return errors.WithStack(scanfile(path, d.Name(), option))
	})
}

func scanfile(filePath string, filename string, option gyazo.UploadOption) error {
	if !isSupportedImageFile(filename) {
		log.Warn("skip file because of unsupported file type", "filepath", filePath)
		return nil
	}
	return errors.WithStack(upload(filePath, option))
}

func upload(filePath string, option gyazo.UploadOption) error {
	return errors.WithStack(gyazo.DefaultClient().Upload(filePath, option))
}
