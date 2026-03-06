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
	CreatePrivateTx(ctx context.Context, arg *CreatePrivateTxParams) (CreatePrivateTxResult, error)
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

type CreatePrivateTxParams struct {
	UserId   int64 `json:"user_id"`
	FriendId int64 `json:"friend_id"`
}

type CreatePrivateTxResult struct {
	Conversation Conversation       `json:"conversation"`
	Member1      ConversationMember `json:"member1"`
	Member2      ConversationMember `json:"member2"`
}

func (store *SQLStore) CreatePrivateTx(ctx context.Context, arg *CreatePrivateTxParams) (CreatePrivateTxResult, error) {
	var result CreatePrivateTxResult

	err := store.execTx(ctx, func(q *Queries) error {

		conversation, err := q.CreateConversation(ctx, CreateConversationParams{
			Type: 1, // 1=私聊 2=群聊
			Name: pgtype.Text{
				Valid: false,
			},
			OwnerID: pgtype.Int8{
				Valid: false,
			},
		})
		message, err := q.CreateMessage(ctx, CreateMessageParams{
			ConversationID: conversation.ID,
			SenderID:       arg.UserId,
			MsgType:        1,
			Content:        "Private conversation created",
		})
		if err != nil {
			return err
		}

		result.Conversation = conversation

		// 2. 创建第一个用户的会话成员（修正参数类型）
		result.Member1, err = q.CreateConversationMember(ctx, CreateConversationMemberParams{
			ConversationID: conversation.ID,
			UserID:         arg.UserId,
			Role:           3,
			LastReadMessageID: pgtype.Int8{
				Int64: message.ID,
				Valid: true,
			},
		})
		if err != nil {
			return err // 事务回滚
		}

		// 3. 创建第二个用户的会话成员（补充缺失的逻辑）
		result.Member2, err = q.CreateConversationMember(ctx, CreateConversationMemberParams{
			ConversationID: conversation.ID,
			UserID:         arg.FriendId,
			Role:           3,
			LastReadMessageID: pgtype.Int8{
				Int64: message.ID,
				Valid: true,
			},
		})
		if err != nil {
			return err // 事务回滚
		}

		return nil // 事务执行成功
	})

	// 返回结果和错误
	return result, err
}
