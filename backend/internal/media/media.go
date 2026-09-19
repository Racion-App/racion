// Package media — фото пользователей: приём файла, пережатие в WebP двух размеров и загрузка в S3 (MinIO).
// Публичные ссылки — через PublicURL (в докере это /media, который nginx проксирует в MinIO).
package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"
	"time"

	"github.com/gen2brain/webp"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var (
	ErrDisabled = errors.New("media: storage not configured")
	ErrBadImage = errors.New("media: not an image")
	ErrTooBig   = errors.New("media: too big")
)

const (
	MaxUpload = 12 << 20 // байт на файл
	largeSide = 1400     // длинная сторона большой версии
	thumbSide = 480
	quality   = 82
)

type Config struct {
	Endpoint  string // minio:9000 или s3.example.com
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool   // https к хранилищу
	PublicURL string // база публичных ссылок: /media или https://cdn.example.com/racion
}

type Store struct {
	cl     *minio.Client
	bucket string
	public string
}

// New — клиент хранилища; пустой Endpoint — фото выключены.
func New(cfg Config) (*Store, error) {
	if cfg.Endpoint == "" {
		return nil, nil
	}
	cl, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.Secure})
	if err != nil {
		return nil, err
	}
	return &Store{cl: cl, bucket: cfg.Bucket, public: strings.TrimRight(cfg.PublicURL, "/")}, nil
}

func (s *Store) Enabled() bool { return s != nil }

// Init создаёт бакет и открывает чтение всем: ссылки на фото публичные, как и страницы рецептов.
func (s *Store) Init(ctx context.Context) error {
	if s == nil {
		return nil
	}
	ok, err := s.cl.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		if err := s.cl.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, s.bucket)
	return s.cl.SetBucketPolicy(ctx, s.bucket, policy)
}

// Photo — что получилось: большая версия и миниатюра.
type Photo struct {
	URL   string `json:"url"`
	Thumb string `json:"thumb"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// Upload читает картинку (jpeg/png/gif/webp), сжимает в WebP и кладёт под kind/<id>.webp и kind/<id>_s.webp.
func (s *Store) Upload(ctx context.Context, kind string, r io.Reader) (Photo, error) {
	if s == nil {
		return Photo{}, ErrDisabled
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxUpload+1))
	if err != nil {
		return Photo{}, err
	}
	if len(raw) > MaxUpload {
		return Photo{}, ErrTooBig
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Photo{}, ErrBadImage
	}
	large := fit(img, largeSide)
	thumb := fit(img, thumbSide)
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	id := time.Now().UTC().Format("2006/01/") + hex.EncodeToString(b)
	key := kind + "/" + id + ".webp"
	keyS := kind + "/" + id + "_s.webp"
	if err := s.put(ctx, key, large); err != nil {
		return Photo{}, err
	}
	if err := s.put(ctx, keyS, thumb); err != nil {
		return Photo{}, err
	}
	bnd := large.Bounds()
	return Photo{URL: s.public + "/" + key, Thumb: s.public + "/" + keyS, W: bnd.Dx(), H: bnd.Dy()}, nil
}

// Owns — ссылка ведёт в наше хранилище (чтобы в профиль нельзя было подставить чужой адрес).
func (s *Store) Owns(url string) bool {
	return s != nil && url != "" && strings.HasPrefix(url, s.public+"/") && !strings.Contains(url, "..")
}

func (s *Store) put(ctx context.Context, key string, img image.Image) error {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: quality, Method: 4}); err != nil {
		return err
	}
	_, err := s.cl.PutObject(ctx, s.bucket, key, bytes.NewReader(buf.Bytes()), int64(buf.Len()), minio.PutObjectOptions{ContentType: "image/webp", CacheControl: "public, max-age=31536000, immutable"})
	return err
}

// fit уменьшает картинку так, чтобы длинная сторона стала side (меньше — не увеличиваем).
func fit(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		if _, ok := img.(*image.RGBA); ok {
			return img
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	if w >= h {
		h = h * side / w
		w = side
	} else {
		w = w * side / h
		h = side
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
