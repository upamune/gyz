package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/cockroachdb/errors"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	"github.com/upamune/gyz/internal/gyazo"
)

func buildGyazoUploadOption() (gyazo.UploadOption, error) {
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

func uploadCommandHandler(cmd *cobra.Command, args []string) error {
	option, err := buildGyazoUploadOption()
	if err != nil {
		return errors.WithStack(err)
	}
	p := pool.New().WithErrors().WithContext(cmd.Context()).WithMaxGoroutines(5)
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

			return errors.WithStack(scanfile(arg, info, option))
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
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return errors.WithStack(scandir(path, option))
		}
		return errors.WithStack(scanfile(path, info, option))
	})
}

func scanfile(filePath string, info os.FileInfo, option gyazo.UploadOption) error {
	if !isSupportedImageFile(info.Name()) {
		log.Warn("skip file because of unsupported file type", "filepath", filePath)
		return nil
	}
	return errors.WithStack(upload(filePath, option))
}

func upload(filePath string, option gyazo.UploadOption) error {
	return errors.WithStack(gyazo.DefaultClient().Upload(filePath, option))
}
