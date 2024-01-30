package gyazo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/exif2"
	"golang.org/x/oauth2"
)

const AccessTokenEnvName = "GYAZO_ACCESS_TOKEN"

var (
	once          sync.Once
	defaultClient *Client
)

type Client struct {
	httpClient *http.Client
}

func DefaultClient() *Client {
	once.Do(func() {
		oauthClient := oauth2.NewClient(
			context.TODO(),
			oauth2.StaticTokenSource(&oauth2.Token{
				// TODO: 環境変数以外から取得する
				AccessToken: os.Getenv(AccessTokenEnvName),
			}),
		)
		defaultClient = &Client{
			httpClient: oauthClient,
		}
	})
	return defaultClient
}

type UploadOption struct {
	AccessPolicy     string
	MetadataIsPublic bool
	EnableExif       bool
	RefererURL       string
	App              string
	Title            string
	Desc             string
	CreatedAt        time.Time
	CollectionID     string
}

type uploadResponse struct {
	ImageID      string `json:"image_id"`
	PermalinkURL string `json:"permalink_url"`
	ThumbURL     any    `json:"thumb_url"`
	Type         string `json:"type"`
	Metadata     struct {
		App   any `json:"app"`
		Title any `json:"title"`
		URL   any `json:"url"`
		Desc  any `json:"desc"`
	} `json:"metadata"`
	Ocr struct {
		Locale      string `json:"locale"`
		Description string `json:"description"`
	} `json:"ocr"`
}

func (option UploadOption) toMap(exif *exif2.Exif) map[string]string {
	m := make(map[string]string)
	if option.AccessPolicy != "" {
		m["access_policy"] = option.AccessPolicy
	}
	if option.MetadataIsPublic {
		s := "false"
		if option.MetadataIsPublic {
			s = "true"
		}
		m["metadata_is_public"] = s
	}
	if option.RefererURL != "" {
		m["referer_url"] = option.RefererURL
	}
	if option.App != "" {
		m["app"] = option.App
	}
	if option.Title != "" {
		m["title"] = option.Title
	}
	if option.Desc != "" {
		m["desc"] = option.Desc
	}
	if !option.CreatedAt.IsZero() {
		m["created_at"] = fmt.Sprintf("%d", option.CreatedAt.Unix())
	}
	if option.CollectionID != "" {
		m["collection_id"] = option.CollectionID
	}

	// NOTE: exif情報が有効な時だけ上書きする
	if exif != nil {
		if !exif.CreateDate().IsZero() {
			d := exif.CreateDate()
			sec := time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), time.Local).Unix()
			m["created_at"] = fmt.Sprintf("%d", sec)
		}
		desc := m["desc"]
		if desc == "" {
			m["desc"] = exif.String()
		} else {
			m["desc"] = desc + "\n\n" + exif.String()
		}
	}
	return m
}

func (c *Client) Upload(filePath string, option UploadOption) (string, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer file.Close()

	var exif *exif2.Exif
	if option.EnableExif {
		file, err := os.Open(filePath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		defer file.Close()

		e, err := imagemeta.Decode(file)
		if err != nil {
			return "", errors.WithStack(err)
		}
		exif = &e
	}

	part, err := writer.CreateFormFile("imagedata", file.Name())
	if err != nil {
		return "", errors.WithStack(err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", errors.WithStack(err)
	}

	for k, v := range option.toMap(exif) {
		if err := writer.WriteField(k, v); err != nil {
			return "", errors.WithStack(err)
		}
	}

	if err := writer.Close(); err != nil {
		return "", errors.WithStack(err)
	}

	const url = "https://upload.gyazo.com/api/upload"

	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return "", errors.WithStack(err)
	}
	req.Header.Add("Content-Type", writer.FormDataContentType())

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.WithStack(err)
	}

	var resp uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return "", errors.WithStack(err)
	}

	return resp.PermalinkURL, nil
}
