package api

import (
	"context"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type storeMessageParams struct {
	ConversationID int64
	SenderID       int64
	MsgType        int16
	Content        string
}

type storeMessageResult struct {
	ConversationID int64
	SenderID       int64
	MsgType        int16
	Content        string
	CreatedAt      pgtype.Timestamptz
}

func (s *Server) storeMessage(c context.Context, arg storeMessageParams) (msg storeMessageResult, err error) {
	message, err := s.store.CreateMessage(c, db.CreateMessageParams{
		ConversationID: arg.ConversationID,
		SenderID:       arg.SenderID,
		MsgType:        arg.MsgType,
		Content:        arg.Content,
	})
	if err != nil {
		return storeMessageResult{}, err
	}
	msg = storeMessageResult{
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		MsgType:        message.MsgType,
		Content:        message.Content,
		CreatedAt:      message.CreatedAt,
	}
	return msg, nil
}

type loadHistoryParams struct {
	ConversationID int64
	CursorID       pgtype.Int8
	Limit          int32
}

type loadHistoryResult struct {
	msgs []storeMessageResult
}

func geneMessageResult(msgs []db.Message) []storeMessageResult {
	var result []storeMessageResult
	for _, msg := range msgs {
		result = append(result, storeMessageResult{
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			MsgType:        msg.MsgType,
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt,
		})
	}
	return result
}

func (s *Server) loadHistory(c context.Context, arg loadHistoryParams) (msg loadHistoryResult, err error) {
	messages, err := s.store.ListHistoryMessages(c, db.ListHistoryMessagesParams{
		ConversationID: arg.ConversationID,
		CursorID:       arg.CursorID,
		Limit:          arg.Limit,
	})
	if err != nil {
		return loadHistoryResult{}, err
	}
	result := loadHistoryResult{
		msgs: geneMessageResult(messages),
	}
	return result, nil
}
