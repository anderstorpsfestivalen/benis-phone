package filesync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Config holds the connection details for the R2 bucket we sync down from.
// R2 is S3-compatible, so we reuse aws-sdk-go but force path-style
// addressing and point Endpoint at the account's R2 host. Region is
// ignored by R2; "auto" satisfies the SDK's required-non-empty check.
type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	AccountID       string // Cloudflare account UUID
	Bucket          string
	// Prefix selects which keys in the bucket get mirrored to disk. Empty
	// means "every object in the bucket".
	Prefix string
}

type sync struct {
	cfg        Config
	session    *session.Session
	svc        *s3.S3
	downloader *s3manager.Downloader
	path       string
}

// Create builds an R2-backed file syncer. The returned sync mirrors
// cfg.Bucket (or the cfg.Prefix subtree of it) into a local directory when
// Start is called.
func Create(cfg Config) (sync, error) {
	if cfg.AccountID == "" {
		return sync{}, fmt.Errorf("filesync: R2 AccountID is required")
	}
	if cfg.Bucket == "" {
		return sync{}, fmt.Errorf("filesync: R2 Bucket is required")
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("auto"),
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return sync{}, err
	}

	return sync{
		cfg:        cfg,
		session:    sess,
		svc:        s3.New(sess),
		downloader: s3manager.NewDownloader(sess),
	}, nil
}

func (f *sync) Start(path string) {
	if f.cfg.SecretAccessKey == "" {
		log.Error("Empty R2 creds, cannot sync")
		return
	}

	log.WithFields(log.Fields{
		"Bucket":       f.cfg.Bucket,
		"Endpoint":     fmt.Sprintf("%s.r2.cloudflarestorage.com", f.cfg.AccountID),
		"Prefix":       f.cfg.Prefix,
		"Local folder": path,
	}).Info("Starting R2 sync")

	f.path = path
	if _, err := os.Stat(f.path); os.IsNotExist(err) {
		log.Info("Folder '" + f.path + "' does not exist, creating.")
		os.MkdirAll(f.path, os.ModePerm)
	}

	input := &s3.ListObjectsInput{Bucket: &f.cfg.Bucket}
	if f.cfg.Prefix != "" {
		input.Prefix = aws.String(f.cfg.Prefix)
	}
	err := f.svc.ListObjectsPages(input, func(p *s3.ListObjectsOutput, last bool) bool {
		prefix := ""
		if p.Prefix != nil {
			prefix = *p.Prefix
		}
		for _, obj := range p.Contents {
			rel := strings.Replace(*obj.Key, prefix, "", 1)
			if rel == "" {
				continue
			}
			localPath := filepath.Join(f.path, rel)
			if _, err := os.Stat(localPath); err == nil {
				continue // already on disk
			} else if !os.IsNotExist(err) {
				log.WithError(err).WithField("path", localPath).Warn("stat failed")
				continue
			}

			log.WithFields(log.Fields{
				"Object":      *obj.Key,
				"Destination": localPath,
			}).Info("Downloading object")

			// A trailing slash in the key denotes a directory marker — just
			// create the directory and move on.
			if strings.HasSuffix(*obj.Key, "/") {
				os.MkdirAll(filepath.Dir(localPath), os.ModePerm)
			} else {
				f.download(*obj.Key, localPath)
			}
		}
		return true
	})
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("R2 sync completed")
}

func (f *sync) download(key, localPath string) {
	os.MkdirAll(filepath.Dir(localPath), os.ModePerm)
	nf, err := os.Create(localPath)
	if err != nil {
		log.WithFields(log.Fields{
			"Filename": localPath,
			"Err":      err,
		}).Error("Failed to create file")
		return
	}
	defer nf.Close()

	n, err := f.downloader.Download(nf, &s3.GetObjectInput{
		Bucket: aws.String(f.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.WithError(err).WithField("key", key).Error("Failed to download file")
		return
	}
	log.WithField("Bytes", n).Info("Downloaded " + key)
}
