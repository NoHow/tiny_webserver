package main

import (
	"context"
	"encoding/json"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type githubUserData struct {
	Login string `json:"login"`
	FirstName string
	LastName string
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

func loadUserData(data []byte) bool {
	var userData githubUserData
	json.Unmarshal(data, &userData)

	gUserData.Login = userData.Login
	gUserData.AvatarUrl = userData.AvatarUrl

	if len(gUserData.Login) != 0 {
		gUserData.IsLoggined = true
		return true
	}
	return false
}

func githubHandler(w http.ResponseWriter, r *http.Request, title string) {
	ctx := context.Background()

	code := r.FormValue("code")
	stateCheck := r.FormValue("state")
	if len(code) == 0 || stateCheck != randomStateString {
		log.Fatal("Something wrong with authentication response :(")
	}
	log.Printf("Received authorization code - %v", code)

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Retrieved initial access token %v", tok)

	client := conf.Client(ctx, tok)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		log.Fatal(err)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if loadUserData(respBody) {
		userstateFile, err := os.OpenFile("data/userstate.json", os.O_CREATE, 0600)
		if err != nil {
			log.Fatal(err)
		}

		data, err := json.Marshal(gUserData)
		if err != nil {
			log.Fatal(err)
		}
		_, err = userstateFile.Write(data)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Received response with user data %v", string(respBody))
	http.Redirect(w, r, "/profile", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request, title string) {
	if !gUserData.IsLoggined {
		return
	}

	gUserData = UserData{}
	http.Redirect(w, r, "/", http.StatusFound)
}