package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"tinywebserver/utils"
	"github.com/matryer/is"
)

var testProviderName = "memory"
var testCookieName = "testCookieName"
var testMaxSessionLifeTime = int64(3600)

func TestStartAndReadSession(t *testing.T) {
	is := is.New(t)

	sessionManager := NewManager("memory", testCookieName, testMaxSessionLifeTime)

	//Test ReadSession with not valid and non-existing cookies
	tbl := []struct {
			cookieName, cookieValue string
			outSesssion Session
			noOutErr    bool
		}{
			{ testCookieName , utils.RandString(32), nil, false },
			{testCookieName ,utils.RandString(16), nil, false},
			{testCookieName, "", nil, true},
			{ "", "a*8510vlksj-=+/.97/dslkfj&^*hkjfa,.523", nil, false},
			{ utils.RandString(32), utils.RandString(8), nil, false},
		}

	for _, tt := range tbl {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: tt.cookieName, Value: tt.cookieValue})
		session, err := sessionManager.ReadSession(req)
		is.Equal(session, tt.outSesssion)
		if tt.noOutErr {
			is.NoErr(err)
		}
	}

	//Test Start and Read with valid cookie
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	cookieValue := utils.RandString(32)
	req.AddCookie(&http.Cookie{Name: testCookieName, Value: cookieValue})

	_ = sessionManager.StartSession(rec, req)
	_, err := sessionManager.ReadSession(req)
	if err != nil {
		t.Errorf("memory provider should have session with sid %v, but it don't", cookieValue)
	}

	//Test without cookie
	rec = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/", nil)

	initialSessionCount := sessionManager.provider.SessionCount()
	_ = sessionManager.StartSession(rec, req)
	actualSessionCount := sessionManager.provider.SessionCount()
	if initialSessionCount + 1 != actualSessionCount {
		t.Errorf("memory should have %v session after manager started new session but instead it have %v", initialSessionCount + 1, actualSessionCount)
	}
}

func TestDestroySession(t *testing.T) {
	is := is.New(t)

	sessionManager := NewManager("memory", testCookieName, testMaxSessionLifeTime)

	cookies := make([]string, 4)
	for index := 0; index < 4; index++ {
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		cookieValue := utils.RandString(32)
		cookies[index] = cookieValue
		req.AddCookie(&http.Cookie{Name: testCookieName, Value: cookieValue})

		_ = sessionManager.StartSession(rec, req)
	}

	tbl := []struct {
		cookieName, cookieValue string
		shouldDestroy bool
	}{
		{ testCookieName , cookies[0], true },
		{testCookieName ,cookies[1], true},
		{testCookieName, "", false},
		{ "", "a*8510vlksj-=+/.97/dslkfj&^*hkjfa,.523", false},
		{ utils.RandString(32), utils.RandString(8), false},
		{ testCookieName , cookies[2], true },
		{testCookieName ,cookies[3], true },
	}

	initialCount := sessionManager.provider.SessionCount()
	is.Equal(initialCount, len(cookies))

	for _, v := range tbl {
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: v.cookieName, Value: v.cookieValue})
		sessionManager.DestroySession(rec, req)

		if v.shouldDestroy {
			actualCookie := rec.Header().Get("Set-Cookie")
			is.True(len(actualCookie) > 0)
		} else {
			actualCookie := rec.Header().Get("Set-Cookie")
			is.True(len(actualCookie) == 0)
		}
	}
	finalCount := sessionManager.provider.SessionCount()
	is.Equal(finalCount, 0)
}

type newManagerFunc func(string, string, int64) *Manager

func isPanicNewManager(is *is.I ,f newManagerFunc, providerName, cookieName string, maxLifeTime int64) {
	defer func() {
		is.True(recover() != nil)
	} ()

	_ = f(providerName, cookieName, maxLifeTime)
}

func TestNewManager(t *testing.T) {
	is := is.New(t)

	isPanicNewManager(is, NewManager, "nonExistingProvider", testCookieName, 32)
	actualManager := NewManager(testProviderName, testCookieName, testMaxSessionLifeTime)
	is.Equal(actualManager.provider, providers[testProviderName])
	is.Equal(actualManager.cookieName, testCookieName)
	is.Equal(actualManager.maxLifetime, testMaxSessionLifeTime)
}

func isPanicRegister(is *is.I, f func(string, PersistenceProvider), name string, provider PersistenceProvider) {
	defer func() {
		is.True(recover() != nil)
	} ()

	f(name, provider)
}

func TestRegister(t *testing.T) {
	is := is.New(t)

	isPanicRegister(is, Register, testProviderName, nil)
	isPanicRegister(is, Register, testProviderName, &Provider{})

	anotherTestProvider := &Provider{}
	anotherTestProviderName := "test_provider_name"
	Register(anotherTestProviderName, anotherTestProvider)
	is.Equal(providers[anotherTestProviderName], anotherTestProvider)
}