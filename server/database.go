package server

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"os"
	"strings"
	"time"
	"tinywebserver/utils"
)

type twsDB struct {
	db *bolt.DB
}

type dbUserData struct {
	AvatarUrl  string
	AdminRight UserRight
	PostsIDs   []int
}

type dbPost struct {
	postId       int `json:"-"`
	Text         string
	Likes        []string
	CreationDate []byte //Must be specified in twsTimeFormat = "2006-01-02T15:04:05.000Z07:00"
	CreatorId    []byte
	RepostId     int
}

const (
	cUsersBucket = "Users"
	cPostsBucket = "Posts"
	cUserID      = "userID"
)

const (
	cUsersBucketNotExistError = cUsersBucket + " bucket doesn't exist"
	cUserNotExistError        = "user doesn't exist"
	cPostsBucketNotExistError = cPostsBucket + " bucket doesn't exist"
	cPostNotExistError        = "post doesn't exist"
)

func createBucketIfNotExistsOrDie(bucketName []byte, db *bolt.DB) {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}

func InitDB() {
	_, err := os.Stat("data")
	if os.IsNotExist(err) {
		err = os.Mkdir("data", 0700)
		if err != nil {
			log.Fatal(err)
		}
	}
	db, err := bolt.Open("data/tws.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createBucketIfNotExistsOrDie([]byte("PagesData"), db)
	createBucketIfNotExistsOrDie([]byte("Users"), db)
	createBucketIfNotExistsOrDie([]byte("Posts"), db)

	listUsers := flag.Bool("listUsers", false, "Shall we list all of the current users")
	wipeUsers := flag.Bool("wipeUsers", false, "Will wipe all user data")
	wipePosts := flag.Bool("wipePosts", false, "Will wipe all user posts")
	setAdmin := flag.String("setAdmin", "", "Will set user with desired Id as Admin")
	setUser := flag.String("putOnEarth", "", "Set user rights back to the common peasant")
	flag.Parse()
	if *listUsers {
		listAllUsers(db)
		os.Exit(0)
	}
	if *wipeUsers {
		fmt.Println("Are you sure you want to DELETE ALL Users? (Yes or y)")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\r\n", "", -1)
		text = strings.Replace(text, "\n", "", -1)
		text = strings.ToLower(text)
		if strings.Compare(text, "yes") == 0 || strings.Compare(text, "y") == 0 {
			wipeBucket(db, []byte("Users"))
		} else {
			fmt.Println("Please type <yes> or <y> if you want to clean user database!")
		}
		os.Exit(0)
	}
	if *wipePosts {
		fmt.Println("Are you sure you want to DELETE ALL Posts? (Yes or y)")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\r\n", "", -1)
		text = strings.Replace(text, "\n", "", -1)
		text = strings.ToLower(text)
		if strings.Compare(text, "yes") == 0 || strings.Compare(text, "y") == 0 {
			wipeBucket(db, []byte("Posts"))
		} else {
			fmt.Println("Please type <yes> or <y> if you want to clean user database!")
		}
		os.Exit(0)
	}
	if len(*setAdmin) > 0 {
		setUserPrivilege(db, []byte(*setAdmin), ADMIN)
	}
	if len(*setUser) > 0 {
		setUserPrivilege(db, []byte(*setUser), USER)
	}
}

func getBucket(tx *bolt.Tx, bucketName string) *bolt.Bucket {
	return tx.Bucket([]byte(bucketName))
}

func (db *twsDB) GetPage(title string) ([]byte, error) {
	var resultData []byte
	err := db.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("PagesData"))
		getResult := b.Get([]byte(title))
		resultData = append(resultData, getResult...)
		return nil
	})

	return resultData, err
}

func (db *twsDB) SavePage(title string, data []byte) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("PagesData"))
		return bucket.Put([]byte(title), data)
	})
}

func appendPostToUser(tx *bolt.Tx, ownerID []byte, postID int) error {
	appendFunc := func(user *dbUserData) {
		user.PostsIDs = append(user.PostsIDs, postID)
	}
	return updateUser(tx, ownerID, appendFunc)
}

func removePostFromUser(tx *bolt.Tx, ownerID []byte, postID int) error {
	removeFunc := func(user *dbUserData) {
		i, _ := utils.FindInt(user.PostsIDs, postID)
		if i >= 0 {
			//TODO: Quite possible place for a bottleneck, might need rethinking in the future
			user.PostsIDs = append(user.PostsIDs[:i], user.PostsIDs[i+1:]...)
		}
	}
	return updateUser(tx, ownerID, removeFunc)
}

func updateUser(tx *bolt.Tx, ownerID []byte, updateFunc func(user *dbUserData)) error {
	usersBucket := tx.Bucket([]byte("Users"))
	if usersBucket == nil {
		return fmt.Errorf("users bucket doesn't exists")
	}
	userBuf := usersBucket.Get(ownerID)
	if userBuf == nil {
		return fmt.Errorf("user with the owner id of %s doesn't exist", ownerID)
	}
	user := &dbUserData{}
	err := json.Unmarshal(userBuf, user)
	if err != nil {
		return err
	}
	updateFunc(user)
	userBuf, err = json.Marshal(user)
	if err != nil {
		return err
	}

	//Update posts and users buckets
	return usersBucket.Put(ownerID, userBuf)
}

func (db *twsDB) saveUserPost(ownerID []byte, postText string) (postID int, err error) {
	log.Println("saveUserPost()")
	if len(postText) == 0 {
		return 0, fmt.Errorf("post text is empty\n")
	}
	err = db.db.Update(func(tx *bolt.Tx) error {
		//Prepare post for Posts bucket
		postsBucket := tx.Bucket([]byte("Posts"))
		if postsBucket == nil {
			return fmt.Errorf("posts bucket doesn't exists")
		}
		id, err := postsBucket.NextSequence()
		if err != nil {
			return err
		}
		postID = int(id)
		post := dbPost{
			Text:         postText,
			CreationDate: toTwsUTCTime(time.Now()),
			CreatorId:    ownerID,
		}
		buf, err := json.Marshal(post)
		if err != nil {
			return err
		}

		//Add association with the owner of the post
		err = appendPostToUser(tx, ownerID, postID)
		if err != nil {
			return err
		}

		err = postsBucket.Put(utils.Itob(postID), buf)
		if err != nil {
			removePostErr := removePostFromUser(tx, ownerID, postID)
			if removePostErr != nil {
				log.Printf("couldn't roll back appended to the user post")
			}
		}
		return err
	})

	if err != nil {
		postID = 0
	}
	return
}

func (db *twsDB) repostUserPost(postToRepostId []byte, reposterId []byte, reposterText string) (resultPostId int, err error) {
	err = db.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(cPostsBucket))
		if postsBucket == nil {
			return fmt.Errorf(cPostsBucketNotExistError)
		}
		postBuf := postsBucket.Get(postToRepostId)
		if postBuf == nil {
			return fmt.Errorf(cPostNotExistError)
		}
		id, err := postsBucket.NextSequence()
		if err != nil {
			return err
		}
		newPostId := int(id)
		newPost := dbPost{
			Text:         reposterText,
			CreationDate: toTwsUTCTime(time.Now()),
			CreatorId:    reposterId,
			RepostId:     utils.Btoi(postToRepostId),
		}
		newPostBuf, err := json.Marshal(newPost)
		if err != nil {
			return err
		}

		//Add association with the owner of the post
		err = appendPostToUser(tx, reposterId, newPostId)
		if err != nil {
			return err
		}
		err = postsBucket.Put(utils.Itob(newPostId), newPostBuf)
		if err != nil {
			removePostErr := removePostFromUser(tx, reposterId, newPostId)
			if removePostErr != nil {
				log.Printf("couldn't roll back appended to the user post")
			}
		} else {
			resultPostId = newPostId
		}
		return err
	})
	return
}

func (db *twsDB) toggleLikeOnUserPost(ownerID []byte, postID int, likeOwner string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte("Posts"))
		if postsBucket == nil {
			return fmt.Errorf("posts bucket doesn't exists")
		}

		postIDb := utils.Itob(postID)
		buf := postsBucket.Get(postIDb)
		if buf == nil {
			return fmt.Errorf("post of user %s with id of [%v], which was attempted to like by %s was not found", ownerID, postID, likeOwner)
		}
		post := &dbPost{}
		err := json.Unmarshal(buf, post)
		if err != nil {
			return err
		}

		copyIndex, _ := utils.FindString(post.Likes, likeOwner)
		if copyIndex >= 0 {
			lastElem := len(post.Likes) - 1
			post.Likes[copyIndex] = post.Likes[lastElem]
			post.Likes = post.Likes[:lastElem]
		} else {
			post.Likes = append(post.Likes, likeOwner)
		}

		buf, err = json.Marshal(post)
		if err != nil {
			return err
		}
		return postsBucket.Put(postIDb, buf)
	})
}

func (db *twsDB) deleteUserPost(ownerID []byte, postID int) error {
	log.Printf("twsDB::deleteUserPost ownerId - %s, postID - %v", ownerID, postID)
	return db.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte("Posts"))
		if postsBucket == nil {
			return fmt.Errorf("posts bucket doesn't exists")
		}
		err := postsBucket.Delete(utils.Itob(postID))
		if err != nil {
			return err
		}

		err = removePostFromUser(tx, ownerID, postID)
		return err
	})
}

func (db *twsDB) getLatestUserPosts(ownerID []byte, maxPostsToGet int, lastKey int) (posts []dbPost, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte("Users"))
		if usersBucket == nil {
			return fmt.Errorf("users bucket doesn't exists\n")
		}
		userBuf := usersBucket.Get(ownerID)
		if userBuf == nil {
			return fmt.Errorf("user with id %s doesn't exists", ownerID)
		}
		user := &dbUserData{}
		err = json.Unmarshal(userBuf, user)
		if err != nil {
			return err
		}

		postsBucket := tx.Bucket([]byte("Posts"))
		if postsBucket == nil {
			return fmt.Errorf("posts bucket doesn't exists\n")
		}

		i := len(user.PostsIDs) - 1
		if lastKey > 0 {
			i, _ = utils.FindInt(user.PostsIDs, lastKey)
			if i < 0 {
				return fmt.Errorf("sorry, current id doesn't exist")
			}
		}
		for postCount := 0; i >= 0 && postCount < maxPostsToGet; i-- {
			postId := user.PostsIDs[i]
			val := postsBucket.Get(utils.Itob(postId))
			if val == nil {
				log.Printf("post id [%v] is missing from posts bucket!", postId)
				continue
			}
			post := &dbPost{}
			err := json.Unmarshal(val, post)
			if err != nil {
				return err
			}
			post.postId = postId
			posts = append(posts, *post)
			postCount++
		}

		return nil
	})

	log.Printf("tws::getLatestUserPosts() input ownerID - %s, maxPostsToGet - %v\nresult - %+v\n", ownerID, maxPostsToGet, posts)
	return posts, err
}

func (db *twsDB) getUserPost(postID int) (post dbPost, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte("Posts"))
		if postsBucket == nil {
			return fmt.Errorf("posts bucket doesn't exist")
		}

		postJson := postsBucket.Get(utils.Itob(postID))
		err = json.Unmarshal(postJson, &post)
		if err != nil {
			return err
		}
		post.postId = postID
		return nil
	})
	return
}

func (db *twsDB) getUserPosts(postsId []int) ([]dbPost, error) {
	if len(postsId) == 0 {
		return nil, fmt.Errorf("no posts were requested")
	}
	posts := make([]dbPost, len(postsId))
	err := db.db.View(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(cPostsBucket))
		if postsBucket == nil {
			return fmt.Errorf(cPostsBucketNotExistError)
		}
		for i, id := range postsId {
			buf := postsBucket.Get(utils.Itob(id))
			if buf == nil {
				return fmt.Errorf(cPostNotExistError)
			}
			post := dbPost{postId: id}
			err := json.Unmarshal(buf, &post)
			if err != nil {
				return err
			}
			posts[i] = post
		}
		return nil
	})
	if err != nil {
		posts = nil
	}

	log.Printf("twsDB::getUserPosts will return %+v", posts)
	return posts, err
}

func (db *twsDB) getUser(userId string) (dbUser dbUserData, err error) {
	if len(userId) == 0 {
		return dbUserData{}, fmt.Errorf("userId is empty")
	}
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket := getBucket(tx, cUsersBucket)
		if bucket == nil {
			return fmt.Errorf(cUsersBucketNotExistError)
		}
		buf := bucket.Get([]byte(userId))
		if buf == nil {
			return fmt.Errorf(cUserNotExistError)
		}
		return json.Unmarshal(buf, &dbUser)
	})
	log.Printf("twsDB::getUser() will return %+v", dbUser)
	return
}

// SyncUser Add user if doesn't exists or update current database data
func (db *twsDB) SyncUser(userData TwsUserData) (TwsUserData, error) {
	userResultData := userData
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("Users"))
		var dbUser dbUserData
		userDBData := bucket.Get([]byte(userData.Id))
		if userDBData != nil {
			err := json.Unmarshal(userDBData, &dbUser)
			if err != nil {
				return err
			}
			//Fields that we want to pull from the database
			userResultData.AdminRight = dbUser.AdminRight
		}
		//Fields that we want to overwrite
		dbUser.AvatarUrl = userData.AvatarUrl

		dbByteData, err := json.Marshal(dbUser)
		if err != nil {
			log.Fatal(err)
		}
		return bucket.Put([]byte(userData.Id), dbByteData)
	})
	if err != nil {
		return TwsUserData{}, err
	}

	log.Printf("twsDB::SyncUser() result %+v\n", userResultData)
	return userResultData, nil
}

func setUserPrivilege(db *bolt.DB, userId []byte, userRight UserRight) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("Users"))
		user := bucket.Get(userId)
		if user == nil {
			return fmt.Errorf("Coudn't find user with Id %v\n", userId)
		}

		var dbUser dbUserData
		err := json.Unmarshal(user, &dbUser)
		if err != nil {
			log.Fatal(err)
		}

		dbUser.AdminRight = userRight
		user, err = json.Marshal(dbUser)
		return bucket.Put(userId, user)
	})
}

func listAllUsers(db *bolt.DB) error {
	return db.View(func(tx *bolt.Tx) error {
		log.Println("Gonna list all of the current users")
		bucket := tx.Bucket([]byte("Users"))
		cursor := bucket.Cursor()

		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var userData dbUserData
			err := json.Unmarshal(value, &userData)

			if err == nil {
				fmt.Printf("User ID: %s, info: %v\n", key, userData)
			} else {
				fmt.Printf("Something wrong with user data from the user with ID - %s\n", key)
			}
		}
		return nil
	})
}

func wipeBucket(db *bolt.DB, bucketName []byte) error {
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketName)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("All users successfully deleted!")
		return nil
	})
	if err != nil {
		log.Fatal(err)
		return err
	}

	return db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucket([]byte("Users"))
		return err
	})
}
