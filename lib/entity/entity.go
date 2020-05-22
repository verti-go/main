package entity

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	s3 "github.com/aws/aws-sdk-go/service/s3"
	esapi "github.com/elastic/go-elasticsearch/esapi"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

type EntityConfig struct {
	SingularName string
	PluralName   string
	IdKey        string
}

type EntityManager struct {
	Config      EntityConfig
	Loaders     map[string]Loader
	Storages    map[string]Storage
	Authorizers map[string]Authorization
}

type Manager interface {
	Save(entity map[string]interface{}, storage string)
	Load(id string, loader string) map[string]interface{}
	Allow(id string, op string, loader string) (bool, map[string]interface{})
}

type Storage interface {
	Store(id string, entity map[string]interface{})
}

type Loader interface {
	Load(id string) map[string]interface{}
}

type Authorization interface {
	CanWrite(id string, loader Loader) (bool, map[string]interface{})
}

type S3AdaptorConfig struct {
	Bucket  string
	Prefix  string
	Session *session.Session
}

type ElasticAdaptorConfig struct {
	Client *elasticsearch7.Client
	Index  string
}

type OwnerAuthorizationConfig struct {
	UserId string
}

type S3LoaderAdaptor struct {
	Config S3AdaptorConfig
}

type S3StorageAdaptor struct {
	Config S3AdaptorConfig
}

type ElasticStorageAdaptor struct {
	Config ElasticAdaptorConfig
}

type OwnerAuthorizationAdaptor struct {
	Config OwnerAuthorizationConfig
}

func (m EntityManager) Save(entity map[string]interface{}, storage string) {
	id := fmt.Sprint(entity[m.Config.IdKey])
	m.Storages[storage].Store(id, entity)
}

func (m EntityManager) Load(id string, loader string) map[string]interface{} {
	return m.Loaders[loader].Load(id)
}

func (m EntityManager) Allow(id string, op string, loader string) (bool, map[string]interface{}) {
	if op == "write" {
		return m.Authorizers["default"].CanWrite(id, m.Loaders[loader])
	} else {
		return false, nil
	}
}

func (l S3LoaderAdaptor) Load(id string) map[string]interface{} {

	buf := aws.NewWriteAtBuffer([]byte{})

	downloader := s3manager.NewDownloader(l.Config.Session)

	_, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(l.Config.Bucket),
		Key:    aws.String(l.Config.Prefix + "" + id + ".json.gz"),
	})

	if err != nil {
		log.Fatalf("failed to download file, %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		log.Fatal(err)
	}

	defer gz.Close()

	text, _ := ioutil.ReadAll(gz)

	var entity map[string]interface{}
	json.Unmarshal(text, &entity)
	return entity
}

func (s S3StorageAdaptor) Store(id string, entity map[string]interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&entity); err != nil {
		log.Fatal(err)
	}
	var buf2 bytes.Buffer
	gz := gzip.NewWriter(&buf2)
	if _, err := gz.Write(buf.Bytes()); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	uploader := s3manager.NewUploader(s.Config.Session)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:          aws.String(s.Config.Bucket),
		Key:             aws.String(s.Config.Prefix + "" + id + ".json.gz"),
		Body:            &buf2,
		ContentType:     aws.String("application/json"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		log.Fatal(err)
	}
	// @todo: invalidate cloudfront object.
}

func (s ElasticStorageAdaptor) Store(id string, entity map[string]interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(entity); err != nil {
		log.Fatalf("Error encoding body: %s", err)
	}
	req := esapi.IndexRequest{
		Index:      s.Config.Index,
		DocumentID: id,
		Body:       &buf,
		Refresh:    "true",
	}
	_, err := req.Do(context.Background(), s.Config.Client)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
}

func (a OwnerAuthorizationAdaptor) CanWrite(id string, loader Loader) (bool, map[string]interface{}) {
	// log.Printf("Check ownership of %s", id)
	entity := loader.Load(id)
	if entity == nil {
		return false, nil
	}
	userId := fmt.Sprint(entity["userId"])
	// log.Printf("Check Entity Ownership: %s == %s", userId, a.Config.UserId)
	return (userId == a.Config.UserId), entity
}