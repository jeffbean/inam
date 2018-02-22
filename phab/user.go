package phab

import "github.com/etcinit/gonduit/requests"

type UserQueryRequest struct {
	Emails           []string `json:"emails"`
	requests.Request          // Includes __conduit__ field needed for authentication.
}

type UserQueryResult []*User

type User struct {
	Phid     string   `json:"phid"`
	UserName string   `json:"userName"`
	RealName string   `json:"realName"`
	Email    string   `json:"email"`
	Image    string   `json:"image"`
	URI      string   `json:"uri"`
	Roles    []string `json:"roles"`
}
