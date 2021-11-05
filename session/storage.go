package session

import (
	"container/list"
	"sync"
	"time"
)

var provider = &Provider{list: list.New()}

type SessionStore struct {
	sid 			string
	timeAccessed 	time.Time
	value 			map[interface{}]interface{}
}

func (st *SessionStore) Set(key, value interface{}) error {
	st.value[key] = value
	provider.SessionUpdate(st.sid)
	return nil
}

func (st *SessionStore) Get(key interface{}) interface{} {
	provider.SessionUpdate(st.sid)
	if v, ok := st.value[key]; ok {
		return v
	}

	return nil
}

type Provider struct {
	lock 		sync.Mutex
	sessions 	map[string]*list.Element
	list 		*list.List
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