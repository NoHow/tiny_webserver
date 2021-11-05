package server

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type githubUserData struct {
	Login string `json:"login"`
	Email string `json:"email"`
	AvatarUrl string `json:"avatar_url"`
}

type iOauth interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, token *oauth2.Token) iHttpClient
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
}

type twsOauth struct {
	config *oauth2.Config
}

func (oauth *twsOauth) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return oauth.config.Exchange(ctx, code, opts...)
}

func (oauth *twsOauth) 	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return oauth.config.AuthCodeURL(state, opts...)
}

func (oauth *twsOauth) Client(ctx context.Context, token *oauth2.Token) iHttpClient {
	return oauth.config.Client(ctx, token)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

type OauthData struct {
	Auth_github_cid string
	Auth_github_csec string
}

type SuperAdminStruct struct {
	SuperAdminId string `yaml:"super_admin_id"`
}

func loadOauthConfig() iOauth {
	cfg, err := os.ReadFile("config/config.yml")
	if err != nil {
		log.Fatal(err)
	}

	oauth := OauthData{}
	err = yaml.Unmarshal(cfg, &oauth)
	if err != nil {
		log.Fatal(err)
	}

	//TODO: remove hardcoded scopes and URLs
	return &twsOauth{
		config: &oauth2.Config{
			ClientID: oauth.Auth_github_cid,
			ClientSecret: oauth.Auth_github_csec,
			Scopes:			[]string{"user"},
			Endpoint: oauth2.Endpoint{
				AuthURL:	"https://github.com/login/oauth/authorize",
				TokenURL: 	"https://github.com/login/oauth/access_token",
			},
		},
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringRunes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63() % int64(len(letterBytes))]
	}

	return string(b)
}

var randomStateString string

func (env *environment) loginHandler(w http.ResponseWriter, r *http.Request) {
	randomStateString = randStringRunes(32)
	url := env.oauth.AuthCodeURL(randomStateString)
	log.Printf("Visit the URL for the auth dialog: %v", url)

	http.Redirect(w, r, url, http.StatusFound)
}

func loadUserData(dbConn iDB, data []byte) {
	var userData githubUserData
	json.Unmarshal(data, &userData)

	sha1Client := sha1.New()
	sha1Client.Write([]byte(userData.Login))
	gUserData.UserID = hex.EncodeToString(sha1Client.Sum([]byte(userData.Email)))
	gUserData.AvatarUrl = userData.AvatarUrl

	if len(gUserData.UserID) != 0 {
		syncedUserData, err := dbConn.SyncUser(gUserData)
		gUserData = syncedUserData
		if err != nil {
			log.Println(err)
			gUserData.IsLogged = false
			return
		}
		gUserData.IsLogged = true
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request, title string) {
	if !gUserData.IsLogged {
		return
	}

	gUserData = TwsUserData{}
	http.Redirect(w, r, "/", http.StatusFound)
}