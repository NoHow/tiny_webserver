package server

import (
	"context"
	"fmt"
	"github.com/boltdb/bolt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
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
}

type environment struct {
	db iDB
	oauth iOauth
	sessionManager *session.Manager
}

func (env *environment) readUserData(r *http.Request) (userData TwsUserData, err error) {
	session, err := env.sessionManager.ReadSession(r)
	if err == nil {
		userData.FillSessionData(session)
	}
	return
}

func (env *environment) getPageTitle(r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		return "", fmt.Errorf("url path is not valid")
	}

	return m[2], nil
}

func (env *environment) viewHandler(w http.ResponseWriter, r *http.Request) {
	pageTitle, err :=  env.getPageTitle(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageData , err := env.db.GetPage(pageTitle)
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
	pageTitle, err :=  env.getPageTitle(r)
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
	pageTitle, err :=  env.getPageTitle(r)
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
	session.Set("userId", userData.UserID)
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

	var userData TwsUserData
	userData.FillSessionData(session)
	err = templates.ExecuteTemplate(w, "profile.html", userData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type Page struct {
	Title string
	Body  []byte
	UData TwsUserData
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
	UserID    string
	AvatarUrl string
	AdminRight UserRight
	IsLogged   bool
}

func (userData *TwsUserData) FillSessionData(session session.Session) {
	if session == nil {
		return
	}

	ok := false
	userData.UserID, ok = session.Get("userId").(string)
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
}

//func loadPage(title string) (*Page, error) {
//	body, err := GetPage(title)
//	if err != nil {
//		return nil, err
//	}
//	return &Page{Title: title, Body: body, UData: gUserData}, nil
//}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html",p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func cssHandler(w http.ResponseWriter, r *http.Request, title string) {
	filename := r.URL.Path[len("/"):]
	body, err := ioutil.ReadFile(filename)
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	contentType := "Text/css"
	w.Header().Add("Content-Type", contentType)
	_, err = w.Write(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
var validPath = regexp.MustCompile("^/(edit|save|view|test|login)/([a-zA-Z0-9]+)$|[/]|^/(/tmpl/css)/([a-zA-Z0-9]+)")

func init() {
	templatesPath = "tmpl/"
}

func Start() {
	//This cannot be located at start, because we want to overwrite templatesPath for tests
	templates = template.Must(template.ParseFiles(templatesPath + "edit.html", templatesPath + "view.html", templatesPath + "test.html", templatesPath + "profile.html"))

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
		db: &twsDB{db: dbConnection},
		oauth: loadOauthConfig(),
		sessionManager: sessionManager,
	}

	http.HandleFunc("/profile/", env.profileHandler)
	http.HandleFunc("/view/", env.viewHandler)
	http.HandleFunc("/edit/", env.editHandler)
	http.HandleFunc("/save/", env.saveHandler)
	http.HandleFunc("/github", env.githubHandler)
	http.HandleFunc("/login/", env.loginHandler)
	http.HandleFunc("/logout/", env.logoutHandler)
	http.HandleFunc("/tmpl/css/", makeHandler(cssHandler))
	http.HandleFunc("/", makeHandler(rootHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}