package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	CreateGroupTx(ctx context.Context, arg *CreateGroupTxParams) (CreateGroupTxResult, error)
	Querier
}

type SQLStore struct {
	*Queries
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) Store {
	return &SQLStore{
		Queries: New(db),
		db:      db,
	}
}

func (store *SQLStore) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	q := New(tx)
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit(ctx)
}

type CreateGroupTxParams struct {
	OwnerID   int64   `json:"owner_id"`
	GroupName string  `json:"group_name"`
	UserIDs   []int64 `json:"user_ids"`
}

type CreateGroupTxResult struct {
	CM []ConversationMember `json:"users"`
}

func (store *SQLStore) CreateGroupTx(ctx context.Context, arg *CreateGroupTxParams) (CreateGroupTxResult, error) {
	var result CreateGroupTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		group, err := q.CreateConversation(ctx, CreateConversationParams{
			Type: 2, // 2 for group 1 for private
			Name: pgtype.Text{
				String: arg.GroupName,
				Valid:  true,
			},
			OwnerID: pgtype.Int8{
				Int64: arg.OwnerID,
				Valid: true,
			},
		})
		message, err := q.CreateMessage(ctx, CreateMessageParams{
			ConversationID: group.ID,
			SenderID:       arg.OwnerID,
			MsgType:        1,
			Content:        "Group created",
		})
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		result.CM, err = q.BatchCreateMembers(ctx, BatchCreateMembersParams{
			ConversationID: group.ID,
			Column2:        arg.UserIDs,
			Role:           3, // 3 for group member,2 for admin,1 for owner
			LastReadMessageID: pgtype.Int8{
				Int64: message.ID,
				Valid: true,
			},
		})
		return err
	})
	return result, err
}

type UpdateUnreadCountParams struct {
	ConversationID int64 `json:"conversation_id"`
	SenderIDID     int64 `json:"sender_id"`
}
