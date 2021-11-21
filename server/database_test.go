package server

import (
	"github.com/boltdb/bolt"
	"github.com/matryer/is"
	"os"
	"testing"
	"tinywebserver/utils"
)

var defaultTestUserData = TwsUserData{
		UserID:     utils.RandString(16),
		AvatarUrl:  "testAvatarUrl.com",
		AdminRight: 0,
		IsLogged:   false,
	}

func TestSavePost(t *testing.T) {
	is := is.New(t)

	db, err := bolt.Open("test_tws.db", 0600, nil)
	is.NoErr(err)
	defer db.Close()
	defer os.Remove("test_tws.db")
	testDB := twsDB{ db: db }
	
	//Test users bucket doesn't exist scenario
	_, err = testDB.savePost([]byte(defaultTestUserData.UserID), dbPost{})
	is.True(err != nil)

	//Test successful save scenario
	createBucketIfNotExistsOrDie([]byte("Users"), testDB.db)
	createBucketIfNotExistsOrDie([]byte("Posts"), testDB.db)
	_, err = testDB.SyncUser(defaultTestUserData)
	
	testPost := dbPost{
		Text:         "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim ID est laborum.",
	}
	_, err = testDB.savePost([]byte(defaultTestUserData.UserID), testPost)
	is.NoErr(err)
	actualPosts, err := testDB.getLatestUserPosts([]byte(defaultTestUserData.UserID), 20, nil)
	is.NoErr(err)
	is.Equal(len(actualPosts), 1)
	is.Equal(actualPosts[0].Text, testPost.Text)
}

func TestDeleteUserPost(t *testing.T) {
	is := is.New(t)
	db, err := bolt.Open("test_tws.db", 0600, nil)
	is.NoErr(err)
	defer db.Close()
	defer os.Remove("test_tws.db")
	testDB := twsDB{ db: db }

	//Test successful delete scenario
	createBucketIfNotExistsOrDie([]byte("Users"), testDB.db)
	createBucketIfNotExistsOrDie([]byte("Posts"), testDB.db)
	_, err = testDB.SyncUser(defaultTestUserData)
	testPost := dbPost{
		Text:	"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim ID est laborum.",
	}
	postDate, err := testDB.savePost([]byte(defaultTestUserData.UserID), testPost)
	is.NoErr(err)
	err = testDB.deleteUserPost([]byte(defaultTestUserData.UserID), postDate)
	is.NoErr(err)
	actualPosts, err := testDB.getLatestUserPosts([]byte(defaultTestUserData.UserID), 20, nil)
	is.NoErr(err)
	is.Equal(len(actualPosts), 0)
}

func TestLikeUserPost(t *testing.T) {
	is := is.New(t)
	db, err := bolt.Open("test_tws.db", 0600, nil)
	is.NoErr(err)
	defer db.Close()
	defer os.Remove("test_tws.db")
	testDB := twsDB{ db: db }

	//Test successful like and unlike scenario
	createBucketIfNotExistsOrDie([]byte("Users"), testDB.db)
	createBucketIfNotExistsOrDie([]byte("Posts"), testDB.db)
	_, err = testDB.SyncUser(defaultTestUserData)
	testPost := dbPost{
		Text:	"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim ID est laborum.",
	}
	defaultUserID := []byte(defaultTestUserData.UserID)
	postDate, err := testDB.savePost(defaultUserID, testPost)
	is.NoErr(err)

	likeOwner := utils.RandString(16)
	err = testDB.likeUserPost(defaultUserID, postDate, likeOwner)
	is.NoErr(err)

	actualPost, err := testDB.getUserPost(defaultUserID, postDate)
	is.NoErr(err)
	is.Equal(len(actualPost.Likes), 1)
	i, _ := utils.FindString(actualPost.Likes, likeOwner)
	is.True(i >= 0)

	err = testDB.likeUserPost(defaultUserID, postDate, likeOwner)
	is.NoErr(err)
	actualPost, err = testDB.getUserPost(defaultUserID, postDate)
	is.NoErr(err)
	is.Equal(len(actualPost.Likes), 0)
}