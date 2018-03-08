package phab

import (
	"github.com/etcinit/gonduit/requests"
)

type UserQueryRequest struct {
	Usernames        []string `json:"usernames"`
	Emails           []string `json:"emails"`
	RealNames        []string `json:"realnames"`
	PHIDs            []string `json:"phids"`
	IDs              []string `json:"ids"`
	Offset           int      `json:"offset"`
	Limit            int      `json:"limit"`
	requests.Request          // Includes __conduit__ field needed for authentication.
}

type UserQueryResponse []User

type User struct {
	PHID     string   `json:"phid"`
	UserName string   `json:"userName"`
	RealName string   `json:"realName"`
	Email    string   `json:"email"`
	Image    string   `json:"image"`
	URI      string   `json:"uri"`
	Roles    []string `json:"roles"`
}
