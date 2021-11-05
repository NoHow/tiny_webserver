package session

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
	"tinywebserver/utils"
)

type PersistenceProvider interface {
	SessionInit(sid string) (Session, error)
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

func (manager *Manager) Start(w http.ResponseWriter, r *http.Request) (session Session) {
	manager.lock.Lock()
	defer manager.lock.Lock()

	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		sid := utils.RandString(32)
		session, _ = manager.provider.SessionInit(sid)
		cookie := http.Cookie{Name: manager.cookieName, Value: url.QueryEscape(sid), Path: "/", HttpOnly: true, MaxAge: int(manager.maxLifetime)}
		http.SetCookie(w, &cookie)
	} else {
		sid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionRead(sid)
	}
	return
}

func (manager *Manager) Destroy(w http.ResponseWriter, r *http.Request) {
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

func (manager *Manager) GC() {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.provider.SessionGC(manager.maxLifetime)
	time.AfterFunc(time.Duration(manager.maxLifetime), func() { manager.GC() })
}

var providers = make(map[string]PersistenceProvider)

func NewManager(providerName, cookieName string, maxLifetime int64) (*Manager, error) {
	provider, err := providers[providerName]
	if !err {
		return nil, fmt.Errorf("session: unknown persistence provider $q (forgotten import?)", providerName)
	}
	return &Manager{ provider: provider, cookieName: cookieName, maxLifetime: maxLifetime}, nil
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
