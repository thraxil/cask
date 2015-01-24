package main

import (
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
)

type S3Backend struct {
	AccessKey  string
	SecretKey  string
	BucketName string
	bucket     *s3.Bucket
}

func NewS3Backend(access_key, secret_key, bucket string) *S3Backend {
	auth := aws.Auth{
		AccessKey: access_key,
		SecretKey: secret_key,
	}
	// TODO: allow configuration of buckets in other regions
	useast := aws.USEast

	connection := s3.New(auth, useast)
	mybucket := connection.Bucket(bucket)

	return &S3Backend{access_key, secret_key, bucket, mybucket}
}

func (d S3Backend) String() string {
	return "S3"
}

func (s *S3Backend) Write(key Key, r io.Reader) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		log.Println("error writing into buffer")
		log.Println(err)
		return err
	}

	err = s.bucket.Put(key.String(), b, "application/octet", s3.BucketOwnerFull)
	if err != nil {
		log.Println("uh oh. couldn't write to bucket")
		log.Println(err)
		return err
	}
	return nil
}

func (s S3Backend) Read(key Key) ([]byte, error) {
	return s.bucket.Get(key.String())
}

func (s S3Backend) Exists(key Key) bool {
	ls, err := s.bucket.List(key.String(), "", "", 1)
	if err != nil {
		return false
	}
	return len(ls.Contents) == 1
}

func (s *S3Backend) Delete(key Key) error {
	return s.bucket.Del(key.String())
}

func (s *S3Backend) ActiveAntiEntropy(cluster *Cluster, site Site, interval int) {
	log.Println("S3 AAE starting")
	// S3 backend doesn't need verification, just rebalancing

	AAE_OFFSET = rand.Intn(10000)
	var jitter = 1
	for {
		log.Println("AAE starting at the top")

		res, err := s.bucket.List("", "", "", 1000)
		if err != nil {
			log.Fatal(err)
		}
		for _, v := range res.Contents {
			jitter = rand.Intn(5)
			time.Sleep(time.Duration(interval+jitter) * time.Second)

			key, err := KeyFromString(v.Key)
			if err != nil {
				continue
			}
			err = site.Rebalance(*key)
			if err != nil {
				log.Println(err)
			}
		}
	}

}

type S3Verifier struct{}

func (v *S3Verifier) Verify(path string, key Key, h string) error {
	// S3 doesn't need verification
	return nil
}

func (b S3Backend) NewVerifier(c *Cluster) Verifier {
	return &S3Verifier{}
}
