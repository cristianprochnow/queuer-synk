package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PublicationStatus string

const (
	PublicationStatusPending   PublicationStatus = "pending"
	PublicationStatusFailed    PublicationStatus = "failed"
	PublicationStatusPublished PublicationStatus = "published"
)

type PublicationAddData struct {
	Status           PublicationStatus
	ErrorCode        string
	ErrorDescription string
	PostId           int
	IntCredentialId  int
}

type PublicationAddFailedData struct {
	ErrorCode        string
	ErrorDescription string
	PostId           int
	IntCredentialId  int
}

type PublicationAddPublishedData struct {
	PostId          int
	IntCredentialId int
}

type Publication struct {
	db *sql.DB
}

func NewPublication(db *sql.DB) *Publication {
	publication := Publication{db: db}

	return &publication
}

func (p *Publication) Finished(publication PublicationAddPublishedData) (int, error) {
	return p.Add(PublicationAddData{
		Status:          PublicationStatusPublished,
		PostId:          publication.PostId,
		IntCredentialId: publication.IntCredentialId,
	})
}

func (p *Publication) Failed(publication PublicationAddFailedData) (int, error) {
	return p.Add(PublicationAddData{
		Status:           PublicationStatusFailed,
		ErrorCode:        publication.ErrorCode,
		ErrorDescription: publication.ErrorDescription,
		PostId:           publication.PostId,
		IntCredentialId:  publication.IntCredentialId,
	})
}

func (p *Publication) Add(publication PublicationAddData) (int, error) {
	var publicationId int

	columnsList := []string{}
	columnsValues := []any{
		publication.Status,
		publication.PostId,
		publication.IntCredentialId,
	}
	labelList := []string{}

	if publication.ErrorCode != "" {
		columnsList = append(columnsList, "publication_error_code")
		columnsValues = append(columnsValues, publication.ErrorCode)
		labelList = append(labelList, "?")
	}

	if publication.ErrorDescription != "" {
		columnsList = append(columnsList, "publication_error_desc")
		columnsValues = append(columnsValues, publication.ErrorDescription)
		labelList = append(labelList, "?")
	}

	labelListText := ""
	columnsListText := ""

	if len(columnsList) > 0 {
		labelListText += "," + strings.Join(labelList, ",")
		columnsListText += "," + strings.Join(columnsList, ",")
	}

	insertRes, insertErr := p.db.ExecContext(
		context.Background(),
		`INSERT INTO synk.publication (publication_status, post_id, int_credential_id`+columnsListText+`)
        VALUES (?, ?, ?`+labelListText+`)`,
		columnsValues...,
	)

	if insertErr != nil {
		return publicationId, fmt.Errorf("models.publication.add: %s", insertErr.Error())
	}

	id, exception := insertRes.LastInsertId()

	if exception != nil {
		return publicationId, fmt.Errorf("models.publication.add: %s", exception.Error())
	}

	publicationId = int(id)

	return publicationId, nil
}
