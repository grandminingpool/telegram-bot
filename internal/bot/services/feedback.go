package services

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type AddFeedbackPayload struct {
	FirstName     *string
	LastName      *string
	Username      *string
	ReportMessage string
}

type FeedbackService struct {
	pgConn *sqlx.DB
}

func (f *FeedbackService) Add(ctx context.Context, userID int64, payload *AddFeedbackPayload) error {
	if _, err := f.pgConn.ExecContext(ctx, `INSERT INTO user_feedback (
		user_id, 
		first_name, 
		last_name, 
		username, 
		report_message
	) VALUES ($1, $2, $3, $4, $5)`,
		userID,
		payload.FirstName,
		payload.LastName,
		payload.Username,
		payload.ReportMessage,
	); err != nil {
		return fmt.Errorf("failed to add new user (id: %d) feedback: %w", userID, err)
	}

	return nil
}

func NewFeedbackService(pgConn *sqlx.DB) *FeedbackService {
	return &FeedbackService{
		pgConn: pgConn,
	}
}
