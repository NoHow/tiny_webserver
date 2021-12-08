package server

import (
	"context"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/microcosm-cc/bluemonday"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"tinywebserver/session"
)

const twsTimeFormat = "2006-01-02T15:04:05.000Z07:00"

func toTwsUTCTime(time time.Time) []byte {
	return []byte(time.UTC().Format(twsTimeFormat))
}

type iHttpClient interface {
	Get(url string) (resp *http.Response, err error)
}

type iDB interface {
	GetPage(title string) ([]byte, error)
	SavePage(title string, data []byte) error
	SyncUser(userData TwsUserData) (TwsUserData, error)
	getUser(userId string) (dbUserData, error)
	getUserPost(postID int) (post dbPost, err error)
	getLatestUserPosts(ownerID []byte, maxPostsToGet int, lastKey int) (posts []dbPost, err error)
	saveUserPost(ownerID []byte, post string) (postID int, err error)
	deleteUserPost(ownerID []byte, postID int) error
	toggleLikeOnUserPost(ownerID []byte, postID int, likeOwner string) error
}

type environment struct {
	db             iDB
	oauth          iOauth
	sessionManager *session.Manager
	sanitizer      *bluemonday.Policy
}

func (env *environment) readUserData(r *http.Request) (userData TwsUserData, err error) {
	session, err := env.sessionManager.ReadSession(r)
	if err == nil {
		userData.FillSessionData(session)
	}
	return
}

func getPathValue(r *http.Request, pathCheck *regexp.Regexp) (string, error) {
	m := pathCheck.FindStringSubmatch(r.URL.Path)
	if m == nil {
		return "", fmt.Errorf("url path is not valid")
	}

	log.Printf("getPathValue will return %v", m[2])
	return m[2], nil
}

func (env *environment) viewHandler(w http.ResponseWriter, r *http.Request) {
	pageTitle, err := getPathValue(r, validPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageData, err := env.db.GetPage(pageTitle)
	if err != nil {
		http.Redirect(w, r, "/edit/"+pageTitle, http.StatusFound)
		return
	}

	session, err := env.sessionManager.ReadSession(r)
	userData := TwsUserData{}
	if err != nil {
		log.Printf(err.Error())
	} else {
		userData.FillSessionData(session)
	}
	renderTemplate(w, "view", &Page{
		Title: pageTitle,
		Body:  pageData,
		UData: userData,
	})
}

func (env *environment) editHandler(w http.ResponseWriter, r *http.Request) {
	pageTitle, err := getPathValue(r, validPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userData, err := env.readUserData(r)
	if err != nil {
		log.Printf(err.Error())
	}
	page := &Page{Title: pageTitle, UData: userData}
	renderTemplate(w, "edit", page)
}

func (env *environment) saveHandler(w http.ResponseWriter, r *http.Request) {
	pageTitle, err := getPathValue(r, validPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	log.Printf("Current body is - %v", body)
	p := &Page{Title: pageTitle, Body: []byte(body)}
	err = p.save(env.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+pageTitle, http.StatusFound)
}

func (env *environment) githubHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	code := r.FormValue("code")
	stateCheck := r.FormValue("state")
	if len(code) == 0 || stateCheck != randomStateString {
		log.Printf("Something wrong with authentication response: code [%v], state [%v]", code, stateCheck)
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}
	log.Printf("Received authorization code - %v", code)

	tok, err := env.oauth.Exchange(ctx, code)
	if err != nil {
		log.Printf(err.Error())
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}
	log.Printf("Retrieved initial access token %v", tok)

	client := env.oauth.Client(ctx, tok)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		log.Printf(err.Error())
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	log.Printf("Received response with user data %v", string(respBody))
	userData, err := loadUserData(env.db, respBody)
	if err != nil {
		log.Printf(err.Error())
		http.Redirect(w, r, "/index", http.StatusFound)
		return
	}
	session := env.sessionManager.StartSession(w, r)
	log.Println("Kicked off session")
	session.Set("userId", userData.Id)
	session.Set("avatarUrl", userData.AvatarUrl)
	session.Set("adminRight", userData.AdminRight)

	http.Redirect(w, r, "/profile", http.StatusFound)
}

func (env *environment) profileHandler(w http.ResponseWriter, r *http.Request) {
	session, err := env.sessionManager.ReadSession(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	var postsPage ProfilePage
	postsPage.SessionOwnerData.FillSessionData(session)

	postsPage.ProfileOwnerData.Id = postsPage.SessionOwnerData.Id
	postsPage.ProfileOwnerData.AvatarUrl = postsPage.SessionOwnerData.AvatarUrl

	pathCheck := regexp.MustCompile("^/(profile)/([a-zA-Z0-9]*)$")
	userID, err := getPathValue(r, pathCheck)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(userID) > 0 {
		user, err := env.db.getUser(userID)
		if err == nil {
			postsPage.ProfileOwnerData.Id = userID
			postsPage.ProfileOwnerData.AvatarUrl = user.AvatarUrl
		} else {
			log.Println(err)
		}
	}

	//TODO: Implement additional loading for posts
	posts, err := env.db.getLatestUserPosts([]byte(postsPage.ProfileOwnerData.Id), 64, 0)
	for _, p := range posts {
		post := &twsPost{
			OwnerName:   postsPage.ProfileOwnerData.Id,
			OwnerAvatar: postsPage.ProfileOwnerData.AvatarUrl,
			OwnerId:     postsPage.ProfileOwnerData.Id,
		}
		err = post.convertFromDBPost(&p)
		if err != nil {
			log.Println(err)
			continue
		}
		postsPage.Posts = append(postsPage.Posts, *post)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	err = templates.ExecuteTemplate(w, "profile.html", postsPage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (env *environment) composePostHandler(w http.ResponseWriter, r *http.Request) {
	userData, err := env.readUserData(r)
	if err != nil || len(userData.Id) == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if err != nil {
		log.Printf(err.Error())
	}
	renderTemplate(w, "compose_post", &Page{UData: userData})
}

func (env *environment) savePostHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("savePostHandler()")
	userData, err := env.readUserData(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	postTextRaw := r.FormValue("body")
	if len(postTextRaw) > 240 || len(postTextRaw) == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	postTextClean := env.sanitizer.Sanitize(postTextRaw)
	_, err = env.db.saveUserPost([]byte(userData.Id), postTextClean)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/profile/", http.StatusFound)
}

func (env *environment) deletePostHandler(w http.ResponseWriter, r *http.Request) {
	userData, err := env.readUserData(r)
	if err != nil {
		//TODO: What should we do here?
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	postIDBuf, ok := values["postID"]
	log.Printf("postID received = %s\n", postIDBuf)
	if !ok || len(postIDBuf) == 0 {
		http.Error(w, "postID is not valid", http.StatusBadRequest)
		return
	}
	postID, err := strconv.Atoi(postIDBuf[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	post, err := env.db.getUserPost(postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.Compare(string(post.OwnerId), userData.Id) != 0 {
		//TODO: Make access denied window?
		http.Error(w, "only the owner of post can delete it", http.StatusForbidden)
		return
	}
	err = env.db.deleteUserPost([]byte(userData.Id), postID)
	if err != nil {
		log.Println(err)
	}

	//TODO: Redirect is funky, should be replaced with something
	http.Redirect(w, r, "/profile", http.StatusFound)
}

func (env *environment) likePostHandler(w http.ResponseWriter, r *http.Request) {
	userData, err := env.readUserData(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	postIDBuf, ok := values["postID"]
	log.Printf("localId received = %s\n", postIDBuf)
	if !ok || len(postIDBuf) == 0 {
		http.Redirect(w, r, "/", http.StatusBadRequest)
	}
	postID, err := strconv.Atoi(postIDBuf[0])
	postOwnerIDBuf, ok := values["postOwnerID"]
	log.Printf("postOwnerID received = %s\n", postOwnerIDBuf)
	if !ok || len(postIDBuf) == 0 {
		http.Redirect(w, r, "/", http.StatusBadRequest)
	}
	if err != nil {
		http.Redirect(w, r, "/", http.StatusBadRequest)
	}
	err = env.db.toggleLikeOnUserPost([]byte(postOwnerIDBuf[0]), postID, userData.Id)
	if err != nil {
		log.Println(err)
	}

	//TODO: Redirect is funky, should be replaced with something
	http.Redirect(w, r, "/profile/"+postOwnerIDBuf[0], http.StatusFound)
}

type Page struct {
	Title string
	Body  []byte
	UData TwsUserData
}

type twsPost struct {
	PostId       int
	Text         string
	Likes        []string
	CreationDate string
	OwnerId      string
	OwnerName    string
	OwnerAvatar  string
}

func (dest *twsPost) convertFromDBPost(src *dbPost) error {
	if src == nil {
		return fmt.Errorf("received empty post")
	}
	dest.Text = src.Text
	dest.Likes = src.Likes
	dest.CreationDate = string(src.CreationDate)
	dest.PostId = src.postId
	return nil
}

type ProfilePage struct {
	SessionOwnerData TwsUserData
	Posts            []twsPost
	ProfileOwnerData TwsUserData
}

func (p *Page) save(dbConn iDB) error {
	return dbConn.SavePage(p.Title, p.Body)
}

type UserRight int

const (
	USER UserRight = iota
	ADMIN
)

type TwsUserData struct {
	Id         string
	AvatarUrl  string
	AdminRight UserRight
	IsLogged   bool
}

func (userData *TwsUserData) FillSessionData(session session.Session) {
	if session == nil {
		return
	}

	ok := false
	userData.Id, ok = session.Get("userId").(string)
	if !ok {
		log.Printf("no userId information inside session")
	}
	userData.AvatarUrl, ok = session.Get("avatarUrl").(string)
	if !ok {
		log.Printf("no avatarUrl information inside session")
	}
	userData.AdminRight, _ = session.Get("adminRight").(UserRight)
	if !ok {
		log.Printf("no adminRight information inside session")
	}
	userData.IsLogged = true

	log.Printf("Current session data - %+v", userData)
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func cssHandler(w http.ResponseWriter, r *http.Request, title string) {
	fileHandler(w, r, title, "text/css")
}

func iconHandler(w http.ResponseWriter, r *http.Request, title string) {
	fileHandler(w, r, title, "image/png")
}

func fileHandler(w http.ResponseWriter, r *http.Request, title string, contentType string) {
	filename := r.URL.Path[len("/"):]
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", contentType)
	_, err = w.Write(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("file handler successfully returend")
}

func rootHandler(w http.ResponseWriter, r *http.Request, title string) {
	http.Redirect(w, r, "/view/index", http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Trying to process request %s", r.URL)

		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

var templatesPath string
var templates *template.Template
var validPath = regexp.MustCompile("^/(edit|save|view|test|login|compose_post|save_post|delete_post|like_post)/([a-zA-Z0-9]+)$|[/]|^/(/tmpl/css|/img/icons/)/([a-zA-Z0-9]+)")

func init() {
	templatesPath = "tmpl/"
}

func Start() {
	//This cannot be located at start, because we want to overwrite templatesPath for tests
	templates = template.Must(template.ParseFiles(templatesPath+"edit.html", templatesPath+"view.html", templatesPath+"test.html", templatesPath+"profile.html",
		templatesPath+"compose_post.html"))

	InitDB()
	dbConnection, err := bolt.Open("data/tws.db", 0600, nil)
	defer dbConnection.Close()
	if err != nil {
		log.Fatal(err)
	}

	sessionManager := session.NewManager("memory", "twssessionid", 3600)
	if err != nil {
		log.Fatal(err)
	}
	sessionManager.StartGC()
	env := environment{
		db:             &twsDB{db: dbConnection},
		oauth:          loadOauthConfig(),
		sessionManager: sessionManager,
		sanitizer:      bluemonday.StrictPolicy(),
	}

	http.HandleFunc("/profile/", env.profileHandler)
	http.HandleFunc("/compose_post/", env.composePostHandler)
	http.HandleFunc("/save_post/", env.savePostHandler)
	http.HandleFunc("/delete_post/", env.deletePostHandler)
	http.HandleFunc("/like_post/", env.likePostHandler)
	http.HandleFunc("/view/", env.viewHandler)
	http.HandleFunc("/edit/", env.editHandler)
	http.HandleFunc("/save/", env.saveHandler)
	http.HandleFunc("/github", env.githubHandler)
	http.HandleFunc("/login/", env.loginHandler)
	http.HandleFunc("/logout/", env.logoutHandler)
	http.HandleFunc("/tmpl/css/", makeHandler(cssHandler))
	http.HandleFunc("/img/icons/", makeHandler(iconHandler))
	http.HandleFunc("/", makeHandler(rootHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
