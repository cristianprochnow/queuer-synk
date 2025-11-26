package model

import (
	"database/sql"
	"fmt"
	"strings"
	"synk/gateway/app/util"
)

type Posts struct {
	db *sql.DB
}

type PostsList struct {
	PostId         int               `json:"post_id"`
	PostName       string            `json:"post_name"`
	TemplateName   string            `json:"template_name"`
	IntProfileName string            `json:"int_profile_name"`
	CreatedAt      string            `json:"created_at"`
	Status         PublicationStatus `json:"status"`
	PostContent    string            `json:"post_content"`
	TemplateId     int               `json:"template_id"`
	IntProfileId   int               `json:"int_profile_id"`
}

func NewPosts(db *sql.DB) *Posts {
	posts := Posts{db: db}

	return &posts
}

func (p *Posts) List(id []int) ([]PostsList, error) {
	var posts []PostsList

	placeholders := make([]string, len(id))
	args := make([]any, len(id))

	for i, id := range id {
		placeholders[i] = "?"
		args[i] = id
	}

	inClause := strings.Join(placeholders, ",")

	rows, rowsErr := p.db.Query(
		`SELECT post.post_id, post.post_name, post.template_id, template.template_name,
                post.int_profile_id, int_profile.int_profile_name, post.created_at,
                "" status, post.post_content
        FROM post
        LEFT JOIN template ON template.template_id = post.template_id
        LEFT JOIN integration_profile int_profile ON int_profile.int_profile_id = post.int_profile_id
        WHERE post.deleted_at IS NULL AND post.post_id IN (`+inClause+`)
        ORDER BY post.created_at ASC`, args...,
	)

	if rowsErr != nil {
		return nil, fmt.Errorf("models.posts.list: %s", rowsErr.Error())
	}

	defer rows.Close()

	rowsErr = rows.Err()

	if rowsErr != nil {
		return nil, fmt.Errorf("models.posts.list: %s", rowsErr.Error())
	}

	publicationModel := NewPublication(p.db)

	for rows.Next() {
		var post PostsList

		exception := rows.Scan(
			&post.PostId,
			&post.PostName,
			&post.TemplateId,
			&post.TemplateName,
			&post.IntProfileId,
			&post.IntProfileName,
			&post.CreatedAt,
			&post.Status,
			&post.PostContent,
		)

		post.CreatedAt = util.ToTimeBR(post.CreatedAt)

		if exception != nil {
			return nil, fmt.Errorf("models.posts.list: %s", exception.Error())
		}

		statusCount, statusCountErr := publicationModel.CountByPost(post.PostId)

		if statusCountErr != nil {
			return nil, fmt.Errorf("models.posts.list: %s", statusCountErr.Error())
		}

		if statusCount[PublicationStatusFailed] > 0 {
			post.Status = PublicationStatusFailed
		} else if statusCount[PublicationStatusPending] > 0 {
			post.Status = PublicationStatusPending
		} else {
			post.Status = PublicationStatusPublished
		}

		posts = append(posts, post)
	}

	return posts, nil
}
