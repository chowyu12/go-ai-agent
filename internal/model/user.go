package model

import "time"

type Role string

const (
	RoleAdmin Role = "admin"
	RoleGuest Role = "guest"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Role      Role      `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResp struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
}

type CreateUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type UpdateUserReq struct {
	Password *string `json:"password,omitzero"`
	Role     *Role   `json:"role,omitzero"`
	Enabled  *bool   `json:"enabled,omitzero"`
}
