package api

import (
	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	util "github.com/Squidwa2d/IM-system-based-Go/utils"

	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

/*
Login API
*/
type LoginRequest struct {
	Password string `json:"password" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type userResponse struct {
	Username  string
	CreatedAt pgtype.Timestamptz
	AvatarUrl pgtype.Text
	// online, offline, busy
	Status    string
	UpdatedAt pgtype.Timestamptz
}

func newUserResponse(user db.User) userResponse {
	return userResponse{
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		AvatarUrl: user.AvatarUrl,
		Status:    user.Status,
		UpdatedAt: user.UpdatedAt,
	}
}

type loginResponse struct {
	AccessToken           string       `json:"access_token"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	User                  userResponse `json:"user"`
}

func (s *Server) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	//检查用户是否存在
	user, err := s.store.GetUserByUsername(c, req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, err))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	//检查密码是否正确
	if err := util.CheckPasswordHash(req.Password, user.PasswdHash); err != nil {
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}

	//生成token
	accessToken, accessTokenPayload, err := s.tokenMaker.CreateToken(req.Username, s.config.AccessTokenDuration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	refreshToken, refreshTokenPayload, err := s.tokenMaker.CreateToken(req.Username, s.config.RefreshTokenDuration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	//更新用户状态
	if _, err := s.store.UpdataStatus(c, db.UpdataStatusParams{
		ID:     user.ID,
		Status: "online",
	}); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
	}
	//返回用户token
	resp := loginResponse{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshTokenPayload.ExpiredAt.Time,
		AccessTokenExpiresAt:  accessTokenPayload.ExpiredAt.Time,
		User:                  newUserResponse(user),
	}
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Succsessfully logged in",
		Data:    resp,
	})
}

/*

Register API

*/

type RegisterRequest struct {
	Password string `json:"password" binding:"required,min=6"`
	Username string `json:"username" binding:"required,alphanum,min=3,max=20"`
}
type RegisterResponse struct {
	Username  string             `json:"username"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	Status    string             `json:"status"`
}

func (s *Server) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	//将密码转换为哈希值
	hash_passwd, err := util.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	//创建用户
	user, err := s.store.CreateUser(c, db.CreateUserParams{
		Username:   req.Username,
		PasswdHash: hash_passwd,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// 23505 是 PostgreSQL 的 "unique_violation"
			if pgErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	resp := RegisterResponse{
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		Status:    user.Status}
	//返回用户信息
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Succsessfully registered user",
		Data:    resp,
	})
}

/*

Refresh Token API

*/

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	UserName     string `json:"username" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken          string    `json:"access_token"`
	AccessTokenExpiresAt time.Time `json:"access_token_expires_at"`
}

func (s *Server) refreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	//验证token
	payload, err := s.tokenMaker.VerifyToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}

	//验证用户是否匹配
	if payload.Username != req.UserName {
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}

	//生成新的token
	accessToken, accessTokenPayload, err := s.tokenMaker.CreateToken(payload.Username, s.config.AccessTokenDuration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	//返回新的token
	resp := RefreshTokenResponse{
		AccessToken:          accessToken,
		AccessTokenExpiresAt: accessTokenPayload.ExpiredAt.Time,
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Succsessfully refreshed token",
		Data:    resp,
	})

}
