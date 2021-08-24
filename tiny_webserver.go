package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

var gUserData = TwsUserData{}

type Page struct {
	Title string
	Body  []byte
	UData TwsUserData
}

func (p *Page) save() error {
	return SavePage(p.Title, p.Body)
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
	IsLoggined bool
}

func loadPage(title string) (*Page, error) {
	body, err := GetPage(title)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body, UData: gUserData}, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html",p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func profileHandler(w http.ResponseWriter, r *http.Request, title string) {
	if !gUserData.IsLoggined {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	err := templates.ExecuteTemplate(w, "profile.html", gUserData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}

	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
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
	w.Write(body)
}

func testHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage("testdata")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	renderTemplate(w, "test", p)
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

	http.HandleFunc("/profile/", makeHandler(profileHandler))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/test/", makeHandler(testHandler))
	http.HandleFunc("/tmpl/css/", makeHandler(cssHandler))
	http.HandleFunc("/login/", makeHandler(loginHandler))
	http.HandleFunc("/logout/", makeHandler(logoutHandler))
	http.HandleFunc("/github", makeHandler(githubHandler))
	http.HandleFunc("/", makeHandler(rootHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}