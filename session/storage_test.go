package session

import (
	"github.com/matryer/is"
	"testing"
	"time"
	"tinywebserver/utils"
)

var mockCounter = struct {
	updateCount int
}{}

type mockProvider struct{}

func (pdr *mockProvider) SessionInit(sid string) (Session, error) {
	return nil, nil
}

func (pdr *mockProvider) SessionReadOrCreate(sid string) (Session, error) {
	return nil, nil
}

func (pdr *mockProvider) SessionRead(sid string) (Session, error) {
	return nil, nil
}

func (pdr *mockProvider) SessionDestroy(sid string) error {
	return nil
}

func (pdr *mockProvider) SessionGC(maxLifetime int64) {
}

func (pdr *mockProvider) SessionUpdate(sid string) error {
	mockCounter.updateCount++
	return nil
}

func (pdr *mockProvider) SessionCount() int {
	return 0
}

func (pdr *mockProvider) SessionsCleanse() {
}


func TestSessionStorage(t *testing.T) {
	is := is.New(t)

	tbl := []struct {
		key, val string
	} {
		{ utils.RandString(8), utils.RandString(64) },
		{ utils.RandString(8), "123vm,.*)(/23.5" },
		{ utils.RandString(8), ""},
	}

	session := &SessionStore{
		sid:          utils.RandString(32),
		timeAccessed: time.Now(),
		value:        make(map[interface{}]interface{}, 0),
		provider: 	  &mockProvider{},
	}
	is.Equal(session.sid, session.SessionId())

	expectedUpdateCount := 0
	expectedSessionMapSize := 0
	for _, tt := range tbl {
		session.Set(tt.key, tt.val)
		expectedSessionMapSize++
		expectedUpdateCount++
		is.Equal(expectedUpdateCount, mockCounter.updateCount)
		is.Equal(expectedSessionMapSize, len(session.value))
	}

	for _, tt := range tbl {
		value := session.Get(tt.key).(string)
		expectedUpdateCount++
		is.Equal(value, tt.val)
		is.Equal(expectedUpdateCount, mockCounter.updateCount)
	}

	for _, tt := range tbl {
		session.Delete(tt.key)
		expectedSessionMapSize--
		expectedUpdateCount++
		is.Equal(expectedUpdateCount, mockCounter.updateCount)
		is.Equal(expectedSessionMapSize, len(session.value))
	}
}