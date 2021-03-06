package server

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
	"tinywebserver/utils"
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

var randomStateString string

func (env *environment) loginHandler(w http.ResponseWriter, r *http.Request) {
	randomStateString = utils.RandString(32)
	url := env.oauth.AuthCodeURL(randomStateString)
	log.Printf("Visit the URL for the auth dialog: %v", url)

	http.Redirect(w, r, url, http.StatusFound)
}

func (env *environment) logoutHandler(w http.ResponseWriter, r *http.Request) {
	env.sessionManager.DestroySession(w, r)

	http.Redirect(w, r, "/", http.StatusFound)
}

func loadUserData(dbConn iDB, data []byte) (TwsUserData, error) {
	var userData githubUserData
	json.Unmarshal(data, &userData)

	sha1Client := sha1.New()
	sha1Client.Write([]byte(userData.Login))
	var twsUserData TwsUserData
	twsUserData.Id = hex.EncodeToString(sha1Client.Sum([]byte(userData.Email)))
	twsUserData.AvatarUrl = userData.AvatarUrl

	if len(twsUserData.Id) != 0 {
		syncedUserData, err := dbConn.SyncUser(twsUserData)
		return syncedUserData, err
	}

	return TwsUserData{}, fmt.Errorf("couldn't generate User ID")
}