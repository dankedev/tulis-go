package comment

import "github.com/google/uuid"

// CreateCommentReq is the request body for creating a comment.
type CreateCommentReq struct {
	PostID      uuid.UUID  `json:"post_id" validate:"required"`
	AuthorName  string     `json:"author_name" validate:"required,min=2,max=100"`
	AuthorEmail string     `json:"author_email" validate:"required,email,max=255"`
	Content     string     `json:"content" validate:"required,min=1,max=5000"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
}

// UpdateCommentReq is the request body for updating a comment (moderation).
type UpdateCommentReq struct {
	Status string `json:"status" validate:"required,oneof=pending approved spam trashed"`
}

// ListCommentsReq holds query parameters for listing comments.
type ListCommentsReq struct {
	PostID string `query:"post_id"`
	Status string `query:"status"`
	Page   int    `query:"page"`
	Limit  int    `query:"limit"`
}

// CommentResponse is the public response shape.
type CommentResponse struct {
	ID          uuid.UUID          `json:"id"`
	PostID      uuid.UUID          `json:"post_id"`
	AuthorName  string             `json:"author_name"`
	AuthorEmail string             `json:"-"` // never expose email
	Content     string             `json:"content"`
	Status      string             `json:"status"`
	ParentID    *uuid.UUID         `json:"parent_id,omitempty"`
	Children    []CommentResponse  `json:"children,omitempty"`
	CreatedAt   string             `json:"created_at"`
}

// ToResponse converts a Comment entity to a safe API response.
func (c *Comment) ToResponse() CommentResponse {
	resp := CommentResponse{
		ID:         c.ID,
		PostID:     c.PostID,
		AuthorName: c.AuthorName,
		Content:    c.Content,
		Status:     c.Status,
		ParentID:   c.ParentID,
		CreatedAt:  c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if len(c.Children) > 0 {
		resp.Children = make([]CommentResponse, len(c.Children))
		for i, child := range c.Children {
			resp.Children[i] = child.ToResponse()
		}
	}
	return resp
}
