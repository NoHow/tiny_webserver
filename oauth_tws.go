package main

import (
	"context"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
	"io/ioutil"
	"gopkg.in/yaml.v3"
)

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
	log.Printf("Received response with user data %v", string(respBody))
	http.Redirect(w, r, "/view/profile", http.StatusFound)
}