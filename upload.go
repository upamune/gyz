package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/chelnak/ysmrr"
	"github.com/chelnak/ysmrr/pkg/animations"
	"github.com/chelnak/ysmrr/pkg/colors"
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
	exif, err := flags.GetBool("exif")
	if err != nil {
		return gyazo.UploadOption{}, errors.WithStack(err)
	}

	return gyazo.UploadOption{
		App:              app,
		Desc:             desc,
		AccessPolicy:     accessPolicy,
		MetadataIsPublic: metadataIsPublic,
		EnableExif:       exif,
	}, nil

}

func buildGyazoUploadOptionWithInteractive() (gyazo.UploadOption, error) {
	var (
		accessPolicy     = "anyone"
		app              = "gyz"
		metadataIsPublic bool
		exif             bool
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
			huh.NewSelect[bool]().
				Title("Exif").
				Description("EXIF情報を使うか否か").
				Options(huh.NewOptions(true, false)...).
				Value(&exif),
			huh.NewInput().Title("App").Description("キャプチャをしたアプリケーション名").Value(&app),
			huh.NewInput().Title("Description").Description("任意のコメント・タグ").Value(&desc),
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
		EnableExif:       exif,
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

func listTargetFilePaths(targets []string) ([]string, error) {
	var targetFilePaths []string

	for _, target := range targets {
		target := target
		info, err := os.Stat(target)
		if err != nil {
			log.Warn("skip file because of stat error", "err", err, "filepath", target)
			continue
		}

		if info.IsDir() {
			files, err := scandir(target)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			targetFilePaths = append(targetFilePaths, files...)
		}

		if isSupportedImageFile(info.Name()) {
			targetFilePaths = append(targetFilePaths, target)
		}
	}

	return targetFilePaths, nil
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

	targetFilePaths, err := listTargetFilePaths(args)
	if err != nil {
		return errors.WithStack(err)
	}

	sm := ysmrr.NewSpinnerManager(
		ysmrr.WithAnimation(animations.Point),
		ysmrr.WithSpinnerColor(colors.FgHiBlue),
	)
	defer sm.Stop()
	sm.Start()

	p := pool.New().WithErrors().WithContext(cmd.Context()).WithMaxGoroutines(parallel)
	for _, path := range targetFilePaths {
		path := path
		p.Go(func(ctx context.Context) error {
			s := sm.AddSpinner(fmt.Sprintf("uploading %s", path))
			url, err := upload(path, option)
			if err != nil {
				s.UpdateMessagef("failed to upload(%s): %s", path, err)
				s.Error()
				return errors.WithStack(err)
			}
			s.UpdateMessagef("uploaded! %s -> %s", path, url)
			s.Complete()
			return nil
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

func scandir(dir string) ([]string, error) {
	var targetFilePaths []string
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if isSupportedImageFile(path) {
			targetFilePaths = append(targetFilePaths, path)
		}
		return nil
	}); err != nil {
		return nil, errors.WithStack(err)
	}
	return targetFilePaths, nil
}

func upload(filePath string, option gyazo.UploadOption) (string, error) {
	url, err := gyazo.DefaultClient().Upload(filePath, option)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return url, nil
}
