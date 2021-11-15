package session

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type storageProvider interface {
	SessionUpdate(sid string) error
}

type SessionStore struct {
	sid 			string
	timeAccessed 	time.Time
	value 			map[interface{}]interface{}
	provider 		storageProvider
}

func (st *SessionStore) Set(key, value interface{}) error {
	st.value[key] = value
	st.provider.SessionUpdate(st.sid)
	return nil
}

func (st *SessionStore) Get(key interface{}) interface{} {
	st.provider.SessionUpdate(st.sid)
	if v, ok := st.value[key]; ok {
		return v
	}

	return nil
}

func (st *SessionStore) Delete(key interface{}) error {
	delete(st.value, key)
	st.provider.SessionUpdate(st.sid)
	return nil
}

func(st *SessionStore) SessionId() string {
	return st.sid
}

type Provider struct {
	lock 		sync.Mutex
	sessions 	map[string]*list.Element
	list 		*list.List
}

func (pdr *Provider) SessionInit(sid string) (Session, error) {
	pdr.lock.Lock()
	defer pdr.lock.Unlock()

	v := make(map[interface{}]interface{}, 0)
	newSession := &SessionStore{
		sid: sid,
		timeAccessed: time.Now(),
		value: v,
		provider: pdr,
	}
	element := pdr.list.PushBack(newSession)
	pdr.sessions[sid] = element
	return newSession, nil
}

func (pdr *Provider) SessionReadOrCreate(sid string) (Session, error) {
	if element, ok := pdr.sessions[sid]; ok {
		return element.Value.(*SessionStore), nil
	} else {
		session, err := pdr.SessionInit(sid)
		return session, err
	}
}

func (pdr *Provider) SessionRead(sid string) (Session, error) {
	if element, ok := pdr.sessions[sid]; ok {
		return element.Value.(*SessionStore), nil
	}

	return nil, fmt.Errorf("session with current sid [%v] deoesn't exist", sid)
}

func (pdr *Provider) SessionDestroy(sid string) error {
	if element, ok := pdr.sessions[sid]; ok {
		delete(pdr.sessions, sid)
		pdr.list.Remove(element)
	}
	return nil
}

func (pdr *Provider) SessionGC(maxLifetime int64) {
	pdr.lock.Lock()
	pdr.lock.Unlock()

	for {
		element, lastTime := pdr.getOldestElementAndUTime()
		if element == nil {
			break
		}
		if (lastTime + maxLifetime) < time.Now().Unix() {
			pdr.SessionDestroy(element.Value.(*SessionStore).sid)
		} else {
			break
		}
	}
}

func (pdr *Provider) SessionUpdate(sid string) error {
	pdr.lock.Lock()
	defer pdr.lock.Unlock()

	if element, ok := pdr.sessions[sid]; ok {
		element.Value.(*SessionStore).timeAccessed = time.Now()
		pdr.list.MoveToFront(element)
	}
	return nil
}

func (pdr *Provider) SessionCount() int {
	return pdr.list.Len()
}

func (pdr *Provider) SessionsCleanse() {
	for {
		element := pdr.list.Back()
		if element == nil {
			break
		}

		pdr.SessionDestroy(element.Value.(*SessionStore).sid)
	}
}

func (pdr *Provider) getOldestElementAndUTime() (*list.Element, int64) {
	element := pdr.list.Back()
	if element == nil {
		return nil, 0
	}

	return element, element.Value.(*SessionStore).timeAccessed.Unix()
}

func init() {
	provider := &Provider{list: list.New()}
	provider.sessions = make(map[string]*list.Element, 0)
	Register("memory", provider)
}