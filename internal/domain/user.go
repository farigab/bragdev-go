package domain

type User struct {
	Login             string `db:"login" json:"login"`
	Name              string `db:"name" json:"name"`
	AvatarURL         string `db:"avatar_url" json:"avatarUrl,omitempty"`
	GitHubAccessToken string `db:"github_access_token" json:"-"`
}

func NewUser(login, name, avatar string) *User {
	return &User{Login: login, Name: name, AvatarURL: avatar}
}
