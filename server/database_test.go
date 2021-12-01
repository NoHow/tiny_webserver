package server

import (
	"github.com/boltdb/bolt"
	"github.com/matryer/is"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"
	"tinywebserver/utils"
)

var defaultTestUserData = TwsUserData{
	UserID:     utils.RandString(16),
	AvatarUrl:  "testAvatarUrl.com",
	AdminRight: 0,
	IsLogged:   false,
}

var testDBFolder = "test_databases"
var dbGeneratorLock = sync.Mutex{}
var dbFilesCounter = 0
var testsReferenceCounter = 0

func generateTestDB(is *is.I, t *testing.T) *bolt.DB {
	dbGeneratorLock.Lock()
	err := os.Mkdir(testDBFolder, 0700)
	if err != nil && !os.IsExist(err){
		log.Fatalln(err)
	}
	testDBName := testDBFolder + "/test_tws" +  strconv.Itoa(dbFilesCounter) + ".db"
	dbFilesCounter++
	testsReferenceCounter++
	defer dbGeneratorLock.Unlock()

	db, err := bolt.Open(testDBName, 0600, nil)
	is.NoErr(err)

	t.Cleanup(func() {
		err := db.Close()
		if err != nil {
			log.Fatalln(err)
		}
		err = os.Remove(testDBName)
		if err != nil {
			log.Fatalln(err)
		}

		dbGeneratorLock.Lock()
		testsReferenceCounter--
		if testsReferenceCounter == 0 {
			err := os.Remove(testDBFolder)
			if err != nil {
				log.Fatalln(err)
			}
		}
		dbGeneratorLock.Unlock()
	})

	return db
}

func TestSavePost(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	db := generateTestDB(is, t)
	testDB := twsDB{ db: db }

	//Test users bucket doesn't exist scenario
	_, err := testDB.saveUserPost([]byte(defaultTestUserData.UserID), "")
	is.True(err != nil)

	//Test successful save scenario
	createBucketIfNotExistsOrDie([]byte("Users"), testDB.db)
	createBucketIfNotExistsOrDie([]byte("Posts"), testDB.db)
	_, err = testDB.SyncUser(defaultTestUserData)

	testPost := dbPost{
		Text:         "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim ID est laborum.",
	}
	_, err = testDB.saveUserPost([]byte(defaultTestUserData.UserID), testPost.Text)
	is.NoErr(err)
	actualPosts, err := testDB.getLatestUserPosts([]byte(defaultTestUserData.UserID), 20, 0)
	is.NoErr(err)
	is.Equal(len(actualPosts), 1)
	is.Equal(actualPosts[0].Text, testPost.Text)
}

func TestDeleteUserPost(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	db := generateTestDB(is,t )
	testDB := twsDB{ db: db }

	//Test successful delete scenario
	createBucketIfNotExistsOrDie([]byte("Users"), testDB.db)
	createBucketIfNotExistsOrDie([]byte("Posts"), testDB.db)
	_, err := testDB.SyncUser(defaultTestUserData)
	testPost := dbPost{
		Text:	"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim ID est laborum.",
	}
	postID, err := testDB.saveUserPost([]byte(defaultTestUserData.UserID), testPost.Text)
	is.NoErr(err)
	err = testDB.deleteUserPost([]byte(defaultTestUserData.UserID), postID)
	is.NoErr(err)
	actualPosts, err := testDB.getLatestUserPosts([]byte(defaultTestUserData.UserID), 20, 0)
	is.NoErr(err)
	is.Equal(len(actualPosts), 0)
}

func TestLikeUserPost(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	db := generateTestDB(is,t )
	testDB := twsDB{ db: db }

	//Test successful like and unlike scenario
	createBucketIfNotExistsOrDie([]byte("Users"), testDB.db)
	createBucketIfNotExistsOrDie([]byte("Posts"), testDB.db)
	_, err := testDB.SyncUser(defaultTestUserData)
	testPost := dbPost{
		Text:	"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim ID est laborum.",
	}
	defaultUserID := []byte(defaultTestUserData.UserID)
	postID, err := testDB.saveUserPost(defaultUserID, testPost.Text)
	is.NoErr(err)

	likeOwner := utils.RandString(16)
	err = testDB.toggleLikeOnUserPost(defaultUserID, postID, likeOwner)
	is.NoErr(err)

	actualPost, err := testDB.getUserPost(postID)
	is.NoErr(err)
	is.Equal(len(actualPost.Likes), 1)
	i, _ := utils.FindString(actualPost.Likes, likeOwner)
	is.True(i >= 0)

	err = testDB.toggleLikeOnUserPost(defaultUserID, postID, likeOwner)
	is.NoErr(err)
	actualPost, err = testDB.getUserPost(postID)
	is.NoErr(err)
	is.Equal(len(actualPost.Likes), 0)
}