package server

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tinywebserver/session"
	"tinywebserver/utils"
	"html/template"
)

type stubDB struct {
	pageData Page
}

func (db *stubDB) GetPage(title string) ([]byte, error) {
	if db.pageData.Title == title {
		return db.pageData.Body, nil
	}

	return nil, fmt.Errorf("couldn't find any page data")
}

func (db *stubDB) SavePage(title string, data []byte) error {
	log.Printf("stubDB::SavePage(%v, %s)\n", title, data)

	if title == "error" {
		return fmt.Errorf("couldn't save page")
	}
	db.pageData.Title = title
	db.pageData.Body = data
	return nil
}

func (db *stubDB) SyncUser(userData TwsUserData) (TwsUserData, error) {
	return userData, nil
}

type stubHttp struct {
	dataForGet string
	errForGet error
}

func (client *stubHttp) Get(url string) (resp *http.Response, err error) {
	return &http.Response{Body: ioutil.NopCloser(strings.NewReader(client.dataForGet)) }, client.errForGet
}

type stubOauth struct{
	ExchangeStubMethod func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)

	httpClientToCreate iHttpClient
}

func (oauth *stubOauth) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if oauth.ExchangeStubMethod != nil {
		return oauth.ExchangeStubMethod(ctx, code, opts...)
	}

	return &oauth2.Token{
		AccessToken: "gho_tnsVGambPaksSAsdQwzGJNo2Bjs2okeLC3n7G4c",
	}, nil
}

func (oauth *stubOauth) Client(ctx context.Context, t *oauth2.Token) iHttpClient {
	return oauth.httpClientToCreate
}

func (oauth *stubOauth) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return ""
}

func checkIfRedirect(rec *httptest.ResponseRecorder, expectedRedirect string, t *testing.T) {
	if rec.Code != http.StatusFound {
		t.Errorf("Expected %v, got %v", http.StatusFound, rec.Code)
	}
	actualRedirect := rec.Header().Get("Location")
	if actualRedirect != expectedRedirect {
		t.Errorf("Expected %s, got %s", expectedRedirect, actualRedirect)
	}
}

func init() {
	templatesPath = "../tmpl/"
	templates = template.Must(template.ParseFiles(templatesPath + "edit.html", templatesPath + "view.html", templatesPath + "test.html", templatesPath + "profile.html"))
}

func TestGetPageTitle(t *testing.T) {
	env := environment{ db: &stubDB{} }

	expectedTitles := []string{ "title1", "", "123ljkjads"}
	testCases := []string{"/view/", "/edit/", "/save/"}
	for index, testCase := range testCases {
		title := expectedTitles[index]
		req := httptest.NewRequest(http.MethodGet, testCase + title, nil)
		resultTitle, err := env.getPageTitle(req)
		if err != nil {
			t.Errorf("Boom")
		}
		if resultTitle != title {
			t.Errorf("Expected %v, got %v", title, resultTitle)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/invalidparent/title_to_die", nil)
	resultTitle, err := env.getPageTitle(req)
	if len(resultTitle) > 0 {
		t.Errorf("Expected empty title, got %v", resultTitle)
	}

	req.URL.Path = "invalidPath"
	resultTitle, err = env.getPageTitle(req)
	if err == nil {
		t.Errorf("Expected error to be non nil")
	}
	if len(resultTitle) > 0 {
		t.Errorf("Expected empty title, got %v", resultTitle)
	}
}

func TestViewHandler(t *testing.T) {
	testTitle := "testpage"
	testBody := []byte("testbody")
	env := environment{
		db: &stubDB{
			pageData: Page{
				Title: testTitle,
				Body: testBody,
			},
	},
		sessionManager: session.NewManager("memory", "twssessionid", 3600),
	}

	testCase := "/view/"
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, testCase + testTitle, nil)

	http.HandlerFunc(env.viewHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected %v, got %v", http.StatusOK, rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), testBody) || !bytes.Contains(rec.Body.Bytes(), []byte(testTitle)) {
		t.Errorf("Expected %s and %s to be on the Page, got %s", testTitle, testBody, rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	emptyEnv := environment{ db: &stubDB{} }

	req.URL.Path = "/view/nopage"
	http.NewRequest(http.MethodGet, testCase + testTitle, nil)
	http.HandlerFunc(emptyEnv.viewHandler).ServeHTTP(rec2, req)
	if rec2.Code != http.StatusFound {
		t.Errorf("Expected %v, got %v", http.StatusFound, rec2.Code)
	}
	expectedRedirect := "/edit/nopage"
	redirect :=	rec2.Header().Get("Location")
	if redirect != expectedRedirect {
		t.Errorf("Expected %s, got %s", expectedRedirect, redirect)
	}

	req.URL.Path = "invalidPath"
	rec3 := httptest.NewRecorder()
	http.HandlerFunc(env.viewHandler).ServeHTTP(rec3, req)
	if rec3.Code != http.StatusBadRequest {
		t.Errorf("Expected %v, got %v", http.StatusBadRequest, rec3.Code)
	}
}

func TestEditHandler(t *testing.T) {
	testTitle := "testpage"
	testBody := []byte("testbody")
	env := environment{
		db: &stubDB{
			pageData: Page{
				Title: testTitle,
				Body: testBody,
			},
		},
		sessionManager: session.NewManager("memory", "twssessionid", 3600),
	}

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/edit/" + testTitle, nil)

	http.HandlerFunc(env.editHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected %v, got %v", http.StatusOK, rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(testTitle)) {
		t.Errorf("Expected %s to be on the Page, got %s", testTitle, rec.Body.String())
	}

	//Invalid path flow
	req.URL.Path = "invalidPath"
	rec2 := httptest.NewRecorder()
	http.HandlerFunc(env.editHandler).ServeHTTP(rec2, req)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("Expected %v, got %v", http.StatusBadRequest, rec2.Code)
	}
}

func TestSaveHandler(t *testing.T) {
	testTitle := "testPage"
	testBody := "testBody"
	env := environment{ db: &stubDB{
		pageData: Page{},
	}}

	//Normal flow
	rec := httptest.NewRecorder()
	r := strings.NewReader("body=" + testBody)
	req, _ := http.NewRequest(http.MethodPost, "/save/" + testTitle, r)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	http.HandlerFunc(env.saveHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Errorf("Expected %v, got %v", http.StatusFound, rec.Code)
	}
	expectedRedirect := "/view/" + testTitle
	redirect :=	rec.Header().Get("Location")
	if redirect != expectedRedirect {
		t.Errorf("Expected %s, got %s", expectedRedirect, redirect)
	}
	savedPage, _ := env.db.GetPage(testTitle)
	if bytes.Compare([]byte(testBody), savedPage) != 0 {
		t.Errorf("Expected request body [%s] and saved page [%s] to be equal", testBody, savedPage)
	}

	//Invalid path flow
	req.URL.Path = "invalidPath"
	rec2 := httptest.NewRecorder()
	http.HandlerFunc(env.saveHandler).ServeHTTP(rec2, req)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("Expected %v, got %v", http.StatusBadRequest, rec2.Code)
	}

	//Database couldn't save data flow
	reqErr, _ := http.NewRequest(http.MethodPost, "/save/" + "error", r)
	rec3 := httptest.NewRecorder()
	http.HandlerFunc(env.saveHandler).ServeHTTP(rec3, reqErr)
	if rec3.Code != http.StatusInternalServerError {
		t.Errorf("Expected %v, got %v", http.StatusInternalServerError, rec3.Code)
	}
}

func TestGithubHandler(t *testing.T) {
	cookieName := "twstestcookie"
	env := environment{
		db: &stubDB{
			pageData: Page{},
		},
		oauth: &stubOauth{
			httpClientToCreate: &stubHttp{
				dataForGet: "{ \"login\" : \"testLogin\", \"email\" : \"testLogin@test.com\", \"avatar_url\" : \"testurl.com\" }",
				errForGet: nil,
			},
		},
		sessionManager: session.NewManager("memory", cookieName, 3600),
	}

	//Normal flow
	rec := httptest.NewRecorder()
	sampleAuthorizationCode := "bd3hkj23dl4ha61f24de87b75c"
	randomStateString = utils.RandString(32)
	req, _ := http.NewRequest(http.MethodGet, "/github?code=" + sampleAuthorizationCode + "&state=" + randomStateString, nil)
	cookieValue := utils.RandString(32)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookieValue})

	http.HandlerFunc(env.githubHandler).ServeHTTP(rec, req)
	checkIfRedirect(rec, "/profile", t)

	expectedAvatarUrl := "testurl.com"
	expectedIsLogged := true
	session, _ := env.sessionManager.ReadSession(req)
	var actualUserData TwsUserData
	actualUserData.FillSessionData(session)
	if len(actualUserData.UserID) == 0 {
		t.Errorf("Expected user ID to be non-empty")
	}
	if actualUserData.AvatarUrl != expectedAvatarUrl || actualUserData.IsLogged != expectedIsLogged {
		t.Errorf("Expected %v, got %v", actualUserData.AvatarUrl, expectedAvatarUrl)
		t.Errorf("Expected %v, got %v", actualUserData.IsLogged, expectedIsLogged)
	}

	//Broken code or state
	rec2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/github?code=" + "&state=" + randomStateString, nil)
	http.HandlerFunc(env.githubHandler).ServeHTTP(rec2, req2)
	checkIfRedirect(rec2, "/index", t)

	//Failed to exchange auth code for token
	env.oauth = &stubOauth{
		ExchangeStubMethod: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
			log.Printf("stubOauth::FailedExchange(%v)\n", code)
			return &oauth2.Token{}, fmt.Errorf("Failed to exchange authentication code for token ")
		},
	}
	rec3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/github?code=" + sampleAuthorizationCode + "&state=" + randomStateString, nil)
	http.HandlerFunc(env.githubHandler).ServeHTTP(rec3, req3)
	checkIfRedirect(rec3, "/index", t)

	//Failed to get user data
	env.oauth = &stubOauth{
		httpClientToCreate: &stubHttp{
			dataForGet: "",
			errForGet: fmt.Errorf("failed to get user data"),
		},
	}
	rec4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodGet, "/github?code=" + sampleAuthorizationCode + "&state=" + randomStateString, nil)
	http.HandlerFunc(env.githubHandler).ServeHTTP(rec4, req4)
	checkIfRedirect(rec3, "/index", t)
}