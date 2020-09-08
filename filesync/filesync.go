package filesync

import (
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

type sync struct {
	s3key      string
	s3secret   string
	bucket     string
	region     string
	path       string
	session    *session.Session
	svc        *s3.S3
	downloader *s3manager.Downloader
}

func Create(s3key string, s3secret string, bucket string, region string) (sync, error) {

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(s3key, s3secret, ""),
	})

	svc := s3.New(sess)

	downloader := s3manager.NewDownloader(sess)

	if err != nil {
		return sync{}, err
	}

	return sync{
		s3key:      s3key,
		s3secret:   s3secret,
		bucket:     bucket,
		region:     region,
		session:    sess,
		svc:        svc,
		downloader: downloader,
	}, nil
}

func (f *sync) Start(path string) {

	if f.s3secret == "" {
		log.Error("Empty S3 creds, cannot sync")
		return
	}

	log.WithFields(log.Fields{
		"Bucket":       f.bucket,
		"Region":       f.region,
		"Local folder": path,
	}).Info("Starting S3 sync")

	f.path = path
	if _, err := os.Stat(f.path); os.IsNotExist(err) {
		log.Info("Folder '" + f.path + "' does not exist, creating.")
		os.MkdirAll(f.path, os.ModePerm)
	}

	err := f.svc.ListObjectsPages(&s3.ListObjectsInput{
		Bucket: &f.bucket,
		Prefix: aws.String("phone/"),
	}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {

		for _, obj := range p.Contents {
			p := strings.Replace(*obj.Key, *p.Prefix, "", 1)
			if p != "" {

				if _, err := os.Stat(f.path + p); os.IsNotExist(err) {
					log.WithFields(log.Fields{
						"Object":      *obj.Key,
						"Destination": f.path + p,
					}).Info("Downloading object")

					pth := *obj.Key
					fm := pth[len(pth)-1:]
					if fm == "/" {
						os.MkdirAll(filepath.Dir(f.path+p), os.ModePerm)
					} else {
						f.download(*obj.Key, p)
					}
				}

			}
		}
		return true
	})
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("S3 sync completed")
}

func (f *sync) download(key string, path string) {

	os.MkdirAll(filepath.Dir(f.path+path), os.ModePerm)
	nf, err := os.Create(f.path + path)
	if err != nil {
		log.WithFields(log.Fields{
			"Filename": f.path + path,
			"Err":      err,
		}).Error("Failed to create file")
	}

	n, err := f.downloader.Download(nf, &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Error("Failed to download file", err)
	}
	log.WithFields(log.Fields{
		"Bytes": n,
	}).Info("Downloaded " + key)
}
