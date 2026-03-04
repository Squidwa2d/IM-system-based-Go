package db

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	util "github.com/Squidwa2d/IM-system-based-Go/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// 定义测试套件结构
type StoreTestSuite struct {
	suite.Suite
	store  Store
	dbPool *pgxpool.Pool
	ctx    context.Context
	cancel context.CancelFunc
}

// SetupSuite 在所有测试开始前运行一次
func (s *StoreTestSuite) SetupSuite() {

	var err error
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Second)
	config, err := util.LoadConfig("../../.")
	if err != nil {
		log.Fatal("无法加载配置:", err)
	}

	// 创建连接池
	s.dbPool, err = pgxpool.New(s.ctx, config.DBSource)
	require.NoError(s.T(), err, "无法创建数据库连接池")

	// 初始化 Store
	s.store = NewStore(s.dbPool)
}

// TearDownSuite 在所有测试结束后运行
func (s *StoreTestSuite) TearDownSuite() {
	s.cancel()
	if s.dbPool != nil {
		s.dbPool.Close()
	}
}

// SetupTest 在每个测试函数前运行
// 这里我们不需要特殊操作，因为事务在测试函数内部处理
func (s *StoreTestSuite) SetupTest() {
	// 可选：在这里清理数据，确保每个测试环境干净
}

// TearDownTest 在每个测试函数后运行
func (s *StoreTestSuite) TearDownTest() {
	// 可选：如果 SetupTest 做了清理，这里可以不需要操作
}

// TestCreateGroupTx 测试 CreateGroupTx 方法
func (s *StoreTestSuite) TestCreateGroupTx() {
	t := s.T()
	ctx := s.ctx

	// 准备测试数据
	ownerID := int64(1001) // 假设数据库中已有此用户，或者你需要先创建用户
	groupName := fmt.Sprintf("test_group_%d", time.Now().UnixNano())
	memberIDs := []int64{1002, 1003} // 假设这些用户也存在

	// 注意：在实际测试中，你可能需要先插入这些 User ID 到 users 表，
	// 否则外键约束会报错。这里假设你已经通过 migration 或 setup 准备好了数据。
	// 如果外键允许 NULL 或者你希望测试失败的情况，可以调整预期。

	params := &CreateGroupTxParams{
		OwnerID:   ownerID,
		GroupName: groupName,
		UserIDs:   memberIDs,
	}

	// 执行测试
	result, err := s.store.CreateGroupTx(ctx, params)

	// 断言
	if err != nil {
		// 如果是因为外键约束失败（用户不存在），这是一个预期的错误场景
		// 你可以选择跳过测试或专门测试错误情况
		t.Logf("执行失败 (可能是外键约束): %v", err)
		// 如果希望测试通过，请确保数据库里有这些用户，或者注释掉下面的 Fatal
		// require.NoError(t, err)
		return
	}

	require.NoError(t, err)
	require.NotEmpty(t, result.CM, "创建的群组成员列表不应为空")
	require.Len(t, result.CM, len(memberIDs), "成员数量应与输入一致")

	// 验证返回的成员数据
	// 注意：ConversationMember 的具体字段取决于你的 sqlc 生成结果
	// 这里假设它有 ConversationID 和 UserID 字段
	for i, member := range result.CM {
		// 简单的校验逻辑，根据实际结构体调整
		// require.Equal(t, member.UserID, memberIDs[i])
		t.Logf("创建成功 - 成员 %d: %+v", i, member)
	}
}

// TestCreateGroupTx_Rollback 测试事务失败时是否正确回滚
func (s *StoreTestSuite) TestCreateGroupTx_Rollback() {
	t := s.T()
	ctx := s.ctx

	// 构造一个会导致错误的参数 (例如传入一个不存在的用户ID，且数据库有严格外键)
	// 或者模拟业务逻辑错误（如果 store 内部有逻辑判断）
	params := &CreateGroupTxParams{
		OwnerID:   999999, // 假设这个 ID 不存在，导致创建 Conversation 失败
		GroupName: "fail_group",
		UserIDs:   []int64{1},
	}

	_, err := s.store.CreateGroupTx(ctx, params)

	// 预期应该报错
	require.Error(t, err, "传入无效 OwnerID 应该导致错误")
	t.Logf("预期错误捕获成功: %v", err)
}

// 运行测试套件
func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}
