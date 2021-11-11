package session

import (
	"net/http"
	"net/url"
	"sync"
	"time"
	"tinywebserver/utils"
)

type PersistenceProvider interface {
	SessionInit(sid string) (Session, error)
	SessionReadOrCreate(sid string) (Session, error)
	SessionRead(sid string) (Session, error)
	SessionDestroy(sid string) error
	SessionGC(maxLifetime int64)
}

type Session interface {
	Set(key, value interface{}) error
	Get(key interface{}) interface{}
	Delete(key interface{}) error
}

type Manager struct {
	cookieName string
	lock sync.Mutex
	provider PersistenceProvider
	maxLifetime int64
}

func (manager *Manager) StartSession(w http.ResponseWriter, r *http.Request) (session Session) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		sid := utils.RandString(32)
		session, _ = manager.provider.SessionInit(sid)
		cookie := http.Cookie{Name: manager.cookieName, Value: url.QueryEscape(sid), Path: "/", HttpOnly: true, MaxAge: int(manager.maxLifetime)}
		http.SetCookie(w, &cookie)
	} else {
		sid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionReadOrCreate(sid)
	}
	return
}

func (manager *Manager) ReadSession(r *http.Request) (Session, error) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		return nil, err
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	return manager.provider.SessionRead(sid)
}

func (manager *Manager) DestroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		return
	} else {
		manager.lock.Lock()
		defer manager.lock.Unlock()

		manager.provider.SessionDestroy(cookie.Value)
		expiration := time.Now()
		cookie := http.Cookie{Name: manager.cookieName, Path: "/", HttpOnly: true, Expires: expiration}
		http.SetCookie(w, &cookie)
	}
}

func (manager *Manager) StartGC() {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.provider.SessionGC(manager.maxLifetime)
	time.AfterFunc(time.Duration(manager.maxLifetime), func() { manager.StartGC() })
}

var providers = make(map[string]PersistenceProvider)

func NewManager(providerName, cookieName string, maxLifetime int64) *Manager {
	provider, ok := providers[providerName]
	if !ok {
		panic("couldn't find provider with provided name")
	}
	return &Manager{ provider: provider, cookieName: cookieName, maxLifetime: maxLifetime}
}

func Register(name string, provider PersistenceProvider) {
	if provider == nil {
		panic("session: Register provider is nil")
	}
	if _, dup := providers[name]; dup {
		panic("sesion: Register called twice for provider " + name)
	}
	providers[name] = provider
}
