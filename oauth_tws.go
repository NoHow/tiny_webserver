package main

import (
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

var conf = &oauth2.Config{
	ClientID: 		"",
	ClientSecret: 	"",
	Scopes:			[]string{"user"},
	Endpoint: oauth2.Endpoint{
		AuthURL:	"https://github.com/login/oauth/authorize",
		TokenURL: 	"https://github.com/login/oauth/access_token",
	},
}


func init() {
	rand.Seed(time.Now().UnixNano())
	loadConfig()
}

type OauthData struct {
	Auth_github_cid string
	Auth_github_csec string
}

type SuperAdminStruct struct {
	SuperAdminId string `yaml:"super_admin_id"`
}

func loadConfig()  {
	cfg, err := os.ReadFile("config/config.yml")
	if err != nil {
		log.Fatal(err)
	}

	oauth := OauthData{}
	err = yaml.Unmarshal(cfg, &oauth)
	if err != nil {
		log.Fatal(err)
	}

	conf.ClientID = oauth.Auth_github_cid
	conf.ClientSecret = oauth.Auth_github_csec
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

func loginHandler(w http.ResponseWriter, r *http.Request, title string) {
	randomStateString = randStringRunes(32)
	url := conf.AuthCodeURL(randomStateString)
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