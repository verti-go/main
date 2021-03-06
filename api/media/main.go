package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"goclassifieds/lib/utils"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// var ginLambda *ginadapter.GinLambda
var handler Handler

type Handler func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)

type ActionContext struct {
	Session    *session.Session
	BucketName string
}

func UploadMediaFile(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {

	res := events.APIGatewayProxyResponse{StatusCode: 403}

	body, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return res, err
	}

	r := http.Request{
		Method: req.HTTPMethod,
		Header: map[string][]string{
			"Content-Type": {req.Headers["Content-Type"]},
		},
		Body: ioutil.NopCloser(bytes.NewBuffer(body)),
	}

	file, header, err := r.FormFile("File")
	if err != nil {
		return res, err
	}

	contentType := header.Header.Get("Content-Type")
	ext, _ := mime.ExtensionsByType(contentType)
	id := utils.GenerateId()

	if contentType == "text/markdown" {
		ext = []string{".md"}
	}

	data := map[string]string{
		"id":                 id,
		"path":               "media/" + id + ext[0],
		"contentType":        contentType,
		"contentDisposition": header.Header.Get("Content-Disposition"),
		"length":             fmt.Sprint(header.Size),
	}

	userId := GetUserId(req)

	uploader := s3manager.NewUploader(ac.Session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(ac.BucketName),
		Key:         aws.String(data["path"]),
		Body:        file,
		ContentType: aws.String(data["contentType"]),
		Metadata:    map[string]*string{"userId": &userId},
	})
	if err != nil {
		return res, err
	}

	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": "application/json",
	}

	body, err = json.Marshal(data)
	res.Body = string(body)

	return res, nil
}

func GetMediaFile(req *events.APIGatewayProxyRequest, ac *ActionContext) (events.APIGatewayProxyResponse, error) {
	res := events.APIGatewayProxyResponse{StatusCode: 500}

	pathPieces := strings.Split(req.Path, "/")
	file := pathPieces[2]

	buf := aws.NewWriteAtBuffer([]byte{})

	downloader := s3manager.NewDownloader(ac.Session)

	_, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(ac.BucketName),
		Key:    aws.String("media/" + file),
	})

	if err != nil {
		return res, err
	}

	ext := strings.Split(pathPieces[len(pathPieces)-1], ".")
	contentType := mime.TypeByExtension(ext[len(ext)-1])

	if ext[len(ext)-1] == "md" {
		contentType = "text/markdown"
	}

	res.StatusCode = 200
	res.Headers = map[string]string{
		"Content-Type": contentType,
	}
	res.Body = base64.StdEncoding.EncodeToString(buf.Bytes())
	res.IsBase64Encoded = true
	return res, nil
}

func InitializeHandler(ac ActionContext) Handler {
	return func(req *events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		if req.HTTPMethod == "POST" {
			return UploadMediaFile(req, &ac)
		} else {
			return GetMediaFile(req, &ac)
		}
	}
}

func GetUserId(req *events.APIGatewayProxyRequest) string {
	userId := ""
	if req.RequestContext.Authorizer["claims"] != nil {
		userId = fmt.Sprint(req.RequestContext.Authorizer["claims"].(map[string]interface{})["sub"])
		if userId == "<nil>" {
			userId = ""
		}
	}
	return userId
}

func init() {
	log.Printf("Gin cold start")
	sess := session.Must(session.NewSession())
	actionContext := ActionContext{
		Session:    sess,
		BucketName: os.Getenv("BUCKET_NAME"),
	}
	handler = InitializeHandler(actionContext)
}

func main() {
	lambda.Start(handler)
}
