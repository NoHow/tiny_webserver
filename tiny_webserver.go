package main

import (
	"context"
	"fmt"
	"github.com/boltdb/bolt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

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
	renderTemplate(w, "view", &Page{
		Title: pageTitle,
		Body:  pageData,
		UData: gUserData,
	})
}

func (env *environment) editHandler(w http.ResponseWriter, r *http.Request) {
	pageTitle, err :=  env.getPageTitle(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page := &Page{Title: pageTitle, UData: gUserData}
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
	loadUserData(env.db, respBody)

	log.Printf("Received response with user data %v", string(respBody))
	http.Redirect(w, r, "/profile", http.StatusFound)
}

var gUserData = TwsUserData{}

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

func profileHandler(w http.ResponseWriter, r *http.Request, title string) {
	if !gUserData.IsLogged {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	err := templates.ExecuteTemplate(w, "profile.html", gUserData)
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

	contentType := "text/css"
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

var templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html", "tmpl/test.html", "tmpl/profile.html"))
var validPath = regexp.MustCompile("^/(edit|save|view|test|login)/([a-zA-Z0-9]+)$|[/]|^/(/tmpl/css)/([a-zA-Z0-9]+)")

func main() {
	
	InitDB()
	dbConnection, err := bolt.Open("data/tws.db", 0600, nil)
	defer dbConnection.Close()
	if err != nil {
		log.Fatal(err)
	}
	env := environment{
		db: &twsDB{db: dbConnection},
		oauth: loadOauthConfig(),
	}

	http.HandleFunc("/profile/", makeHandler(profileHandler))
	http.HandleFunc("/view/", env.viewHandler)
	http.HandleFunc("/edit/", env.editHandler)
	http.HandleFunc("/save/", env.saveHandler)
	http.HandleFunc("/github", env.githubHandler)
	http.HandleFunc("/login/", env.loginHandler)
	http.HandleFunc("/tmpl/css/", makeHandler(cssHandler))
	http.HandleFunc("/logout/", makeHandler(logoutHandler))
	http.HandleFunc("/", makeHandler(rootHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}