package controller

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"synk/gateway/app/model"
)

type Post struct {
	postModel        *model.Posts
	publicationModel *model.Publication
	publisherClient  *http.Client
}

type HandlePostSendResponse struct {
	Resource ResponseHeader                             `json:"resource"`
	Posts    map[int]map[int]HandlePostSendDataResponse `json:"posts"`
}

type HandlePostSendDataResponse struct {
	Resource ResponseHeader `json:"resource"`
	HttpCode int            `json:"http_code"`
	Raw      any            `json:"raw"`
}

type HandlePostDiscordPublishBody struct {
	WebhookUrl string `json:"webhook_url"`
}

type HandlePostTelegramPublishBody struct {
	BotToken string `json:"bot_token"`
	ChatId   string `json:"chat_id"`
}

type HandlePostSendRequest struct {
	Posts []int `json:"posts"`
}

func NewPost(db *sql.DB) *Post {
	post := Post{
		postModel:        model.NewPosts(db),
		publicationModel: model.NewPublication(db),
		publisherClient:  NewPublisherServiceClient(),
	}

	return &post
}

func (p *Post) HandleSend(w http.ResponseWriter, r *http.Request) {
	SetJsonContentType(w)

	response := HandlePostSendResponse{
		Resource: ResponseHeader{
			Ok: true,
		},
		Posts: map[int]map[int]HandlePostSendDataResponse{},
	}

	publisherUrl := strings.TrimSuffix(os.Getenv("PUBLISHER_ENDPOINT"), "/")

	if publisherUrl == "" {
		response.Resource.Ok = false
		response.Resource.Error = "Publisher URL not set"

		WriteErrorResponse(w, response, "/send", response.Resource.Error, http.StatusInternalServerError)

		return
	}

	bodyContent, bodyErr := io.ReadAll(r.Body)

	if bodyErr != nil {
		response.Resource.Ok = false
		response.Resource.Error = "error on read message body"

		WriteErrorResponse(w, response, "/send", response.Resource.Error, http.StatusBadRequest)

		return
	}

	var post HandlePostSendRequest

	jsonErr := json.Unmarshal(bodyContent, &post)

	if jsonErr != nil {
		response.Resource.Ok = false
		response.Resource.Error = "some fields can be in invalid format"

		WriteErrorResponse(w, response, "/send", response.Resource.Error, http.StatusBadRequest)

		return
	}

	if len(post.Posts) == 0 {
		response.Resource.Ok = false
		response.Resource.Error = "`posts` can not be empty"

		WriteErrorResponse(w, response, "/send", response.Resource.Error, http.StatusBadRequest)

		return
	}

	postsDb, postsDbErr := p.postModel.List(post.Posts)

	if postsDbErr != nil {
		response.Resource.Ok = false
		response.Resource.Error = postsDbErr.Error()

		WriteErrorResponse(w, response, "/send", response.Resource.Error, http.StatusInternalServerError)

		return
	}

	if response.Posts == nil {
		response.Posts = make(map[int]map[int]HandlePostSendDataResponse)
	}

	for _, postDb := range postsDb {
		if response.Posts[postDb.PostId] == nil {
			response.Posts[postDb.PostId] = make(map[int]HandlePostSendDataResponse)
		}

		var payload map[string]string
		var endpoint string

		switch postDb.IntCredentialType {
		case model.INT_CREDENTIAL_DISCORD_TYPE:
			var discordBody HandlePostDiscordPublishBody

			json.Unmarshal([]byte(postDb.IntCredentialConfig), &discordBody)

			if discordBody.WebhookUrl == "" {
				response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
					Resource: ResponseHeader{
						Ok:    false,
						Error: postDb.IntCredentialName + " credential is incomplete or invalid",
					},
					HttpCode: http.StatusBadRequest,
				}

				p.publicationModel.Failed(model.PublicationAddFailedData{
					ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
					ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
					PostId:           postDb.PostId,
					IntCredentialId:  postDb.IntCredentialId,
				})

				continue
			}

			payload = map[string]string{
				"message":     postDb.PostContent,
				"webhook_url": discordBody.WebhookUrl,
			}
			endpoint = "discord/publish"
		case model.INT_CREDENTIAL_TELEGRAM_TYPE:
			var telegramBody HandlePostTelegramPublishBody

			json.Unmarshal([]byte(postDb.IntCredentialConfig), &telegramBody)

			if telegramBody.BotToken == "" || telegramBody.ChatId == "" {
				response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
					Resource: ResponseHeader{
						Ok:    false,
						Error: postDb.IntCredentialName + " credential is incomplete or invalid",
					},
					HttpCode: http.StatusBadRequest,
				}

				p.publicationModel.Failed(model.PublicationAddFailedData{
					ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
					ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
					PostId:           postDb.PostId,
					IntCredentialId:  postDb.IntCredentialId,
				})

				continue
			}

			payload = map[string]string{
				"message":   postDb.PostContent,
				"bot_token": telegramBody.BotToken,
				"chat_id":   telegramBody.ChatId,
			}
			endpoint = "telegram/publish"
		}

		if endpoint == "" {
			response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
				Resource: ResponseHeader{
					Ok:    false,
					Error: postDb.IntCredentialName + " credential has invalid type " + postDb.IntCredentialType,
				},
				HttpCode: http.StatusBadRequest,
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})

			continue
		}

		jsonPayload, jsonPayloadErr := json.Marshal(payload)

		if jsonPayloadErr != nil {
			response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
				Resource: ResponseHeader{
					Ok:    false,
					Error: "some fields can be in invalid format on sending message",
				},
				HttpCode: http.StatusBadRequest,
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})

			continue
		}

		publishReq, publishReqErr := http.NewRequest("POST", publisherUrl+"/"+endpoint, bytes.NewBuffer(jsonPayload))

		if publishReqErr != nil {
			response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
				Resource: ResponseHeader{
					Ok:    false,
					Error: "error while setting publish request: " + publishReqErr.Error(),
				},
				HttpCode: http.StatusInternalServerError,
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})

			continue
		}

		publishReq.Header.Set("Accept", "application/json")
		publishReq.Header.Set("Content-Type", "application/json")

		publishResp, publishRespErr := p.publisherClient.Do(publishReq)
		if publishRespErr != nil {
			response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
				Resource: ResponseHeader{
					Ok:    false,
					Error: "error while doing publish request: " + publishReqErr.Error(),
				},
				HttpCode: http.StatusInternalServerError,
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})

			continue
		}
		defer publishResp.Body.Close()

		var publishRespContent HandlePostSendDataResponse

		bodyBytes, readErr := io.ReadAll(publishResp.Body)

		if readErr != nil {
			response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
				Resource: ResponseHeader{
					Ok:    false,
					Error: "error while parsing publish server response: " + readErr.Error(),
				},
				HttpCode: http.StatusInternalServerError,
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})

			continue
		}

		if err := json.Unmarshal(bodyBytes, &publishRespContent); err != nil {
			response.Posts[postDb.PostId][postDb.IntCredentialId] = HandlePostSendDataResponse{
				Resource: ResponseHeader{
					Ok:    false,
					Error: "error while decoding publish server response: " + err.Error(),
				},
				HttpCode: http.StatusInternalServerError,
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})

			continue
		}

		publishRespContent.HttpCode = publishResp.StatusCode
		response.Posts[postDb.PostId][postDb.IntCredentialId] = publishRespContent

		if !response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Ok {
			errorMessage := response.Posts[postDb.PostId][postDb.IntCredentialId].Resource.Error

			if errorMessage == "" {
				errorMessage = fmt.Sprint(response.Posts[postDb.PostId][postDb.IntCredentialId].Raw)
			}

			p.publicationModel.Failed(model.PublicationAddFailedData{
				ErrorCode:        strconv.Itoa(response.Posts[postDb.PostId][postDb.IntCredentialId].HttpCode),
				ErrorDescription: errorMessage,
				PostId:           postDb.PostId,
				IntCredentialId:  postDb.IntCredentialId,
			})
		} else {
			p.publicationModel.Finished(model.PublicationAddPublishedData{
				PostId:          postDb.PostId,
				IntCredentialId: postDb.IntCredentialId,
			})

		}
	}

	WriteStatusResponse(w, response)
}
