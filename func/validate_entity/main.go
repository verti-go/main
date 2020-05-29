package main

import (
	"context"
	"encoding/json"
	"errors"
	"goclassifieds/lib/ads"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/profiles"
	"goclassifieds/lib/utils"
	"goclassifieds/lib/vocab"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-playground/validator/v10"
)

func handler(ctx context.Context, payload *entity.ValidateEntityRequest) (entity.ValidateEntityResponse, error) {
	log.Print("Inside validate")
	log.Printf("Entity: %s", payload.EntityName)

	invalid := entity.ValidateEntityResponse{
		Entity:       payload.Entity,
		Valid:        false,
		Unauthorized: true,
	}

	if payload.UserId == "" {
		return invalid, errors.New("Unauthorized to create entity")
	}

	invalid.Unauthorized = false

	jsonData, err := json.Marshal(payload.Entity)
	if err != nil {
		return invalid, err
	}

	/*var obj ads.Ad
	err = json.Unmarshal(jsonData, &obj)
	if err != nil {
		return invalid, err
	}

	submitted := ads.Submitted

	obj.Id = utils.GenerateId()
	obj.Status = &submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return invalid, err.(validator.ValidationErrors)
	}*/

	var newEntity map[string]interface{}
	if payload.EntityName == "ad" {
		newEntity, err = ValidateAd(jsonData, payload)
	} else if payload.EntityName == "vocabulary" {
		newEntity, err = ValidateVocabulary(jsonData, payload)
	} else if payload.EntityName == "profile" {
		newEntity, err = ValidateProfile(jsonData, payload)
	} else {
		return invalid, errors.New("Entity validation does exist")
	}

	if err != nil {
		return invalid, err
	}

	return entity.ValidateEntityResponse{
		Entity:       newEntity,
		Valid:        true,
		Unauthorized: false,
	}, nil
}

func ValidateAd(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj ads.Ad
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	submitted := ads.Submitted

	obj.Id = utils.GenerateId()
	obj.Status = &submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := ads.ToEntity(&obj)
	return newEntity, nil
}

func ValidateVocabulary(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj vocab.Vocabulary
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	obj.Id = utils.GenerateId()
	obj.UserId = payload.UserId

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := vocab.ToEntity(&obj)
	return newEntity, nil
}

func ValidateProfile(jsonData []byte, payload *entity.ValidateEntityRequest) (map[string]interface{}, error) {
	var deadObject map[string]interface{}

	var obj profiles.Profile
	err := json.Unmarshal(jsonData, &obj)
	if err != nil {
		return deadObject, err
	}

	submitted := profiles.Submitted

	obj.Id = utils.GenerateId()
	obj.Status = &submitted // @todo: Enums not being validated :(
	obj.UserId = payload.UserId
	obj.EntityPermissions = profiles.ProfilePermissions{
		ReadUserIds:   []string{obj.UserId},
		WriteUserIds:  []string{obj.UserId},
		DeleteUserIds: []string{obj.UserId},
	}

	validate := validator.New()
	err = validate.Struct(obj)
	if err != nil {
		return deadObject, err.(validator.ValidationErrors)
	}

	newEntity, _ := profiles.ToEntity(&obj)
	return newEntity, nil
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}
