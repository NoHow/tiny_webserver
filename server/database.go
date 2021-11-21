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
	AvatarUrl string
	AdminRight UserRight
}

type dbPost struct {
	Text  			string
	Likes        	[]string
	creationDate 	time.Time	`json:"-"`	//Must be specified in twsTimeFormat = "2006-01-02T15:04:05.000Z07:00"
}

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

	listUsers 	:= flag.Bool("listUsers", false, "Shall we list all of the current users")
	wipeUsers 	:= flag.Bool("wipeUsers", false, "Will wipe all user data")
	setAdmin 	:= flag.String("setAdmin", "", "Will set user with desired UserID as Admin")
	setUser		:= flag.String("putOnEarth", "", "Set user rights back to the common peasant")
	flag.Parse()
	if *listUsers {
		listAllUsers(db)
		os.Exit(0)
	}
	if *wipeUsers {
		fmt.Println("Are you sure you want to DELETE ALL Users? (Yes or y)")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\r\n", "",-1)
		text = strings.Replace(text, "\n", "",-1)
		text = strings.ToLower(text)
		if strings.Compare(text, "yes") == 0 || strings.Compare(text, "y") == 0 {
			wipeAllUsers(db)
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

func (db *twsDB) getPostsBucket(tx *bolt.Tx, ownerID []byte, shouldCreate bool) (*bolt.Bucket, error) {
	mainBucket := tx.Bucket([]byte("Posts"))
	if mainBucket == nil {
		return nil, fmt.Errorf("posts bucket doesn't exists")
	}
	postsBucket := mainBucket.Bucket(ownerID)
	if postsBucket != nil {
		return postsBucket, nil
	}

	if shouldCreate {
		newUserBucket, err := mainBucket.CreateBucket(ownerID)
		if err != nil {
			return nil, fmt.Errorf("wasn't able to create new user posts bucket")
		}
		return newUserBucket, nil
	}
	return nil, fmt.Errorf("bucket for user %v doesn't exist", ownerID)
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

func (db *twsDB) savePost(ownerID []byte, post dbPost) (saveTime []byte, err error)  {
	err = db.db.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte("Posts"))
		if mainBucket == nil {
			return fmt.Errorf("posts bucket doesn't exists")
		}
		postsBucket := mainBucket.Bucket(ownerID)
		if postsBucket == nil {
			log.Printf("bucket with posts of user %s doesn't exists", ownerID)
			newUserBucket, err := mainBucket.CreateBucket(ownerID)
			if err != nil {
				return fmt.Errorf("wasn't able to create new user posts bucket")
			}
			postsBucket = newUserBucket
		}

		saveTime = toTwsUTCTime(time.Now())
		postJson, err := json.Marshal(post)
		if err != nil {
			return err
		}
		return postsBucket.Put([]byte(saveTime), postJson)
	})
	return
}

func (db *twsDB) likeUserPost(ownerID []byte, postDate []byte, likeOwner string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		bucket, err := db.getPostsBucket(tx, ownerID, false)
		if err != nil {
			return err
		}

		postJson := bucket.Get(postDate)
		if postJson == nil {
			return fmt.Errorf("post of user %s with id of [%s], which was attempted to like by %s was not found", ownerID, postDate, likeOwner)
		}
		post := &dbPost{}
		err = json.Unmarshal(postJson, post)
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

		postJson, err = json.Marshal(post)
		if err != nil {
			return err
		}
		return bucket.Put(postDate, postJson)
	})
}

func (db *twsDB) deleteUserPost(ownerID []byte, postDate []byte) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte("Posts"))
		if mainBucket == nil {
			return fmt.Errorf("posts bucket doesn't exists")
		}
		postsBucket := mainBucket.Bucket(ownerID)
		if postsBucket == nil {
			return fmt.Errorf("bucket with posts of user %s doesn't exists", ownerID)
		}

		return postsBucket.Delete(postDate)
	})
}

func (db *twsDB) getLatestUserPosts(ownerID []byte, maxPostsToGet int, lastKey *time.Time) (posts []dbPost, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket([]byte("Posts"))
		if mainBucket == nil {
			log.Fatalf("posts bucket doesn't exists!")
		}

		postsBucket := mainBucket.Bucket(ownerID)
		if postsBucket == nil {
			return fmt.Errorf("bucket with posts of user %s doesn't exists", ownerID)
		}
		c := postsBucket.Cursor()
		var max, k, v []byte
		if lastKey != nil {
			max = []byte(lastKey.UTC().Format(twsTimeFormat))
			k, v = c.Seek(max)
		} else {
			k, v = c.Last()
		}
		for ; k != nil && len(posts) < maxPostsToGet; k, v = c.Prev() {
			if v == nil {
				return fmt.Errorf("found post key without value, key - %s", k)
			}
			post := &dbPost{}
			err := json.Unmarshal(v, post)
			if err != nil {
				return err
			}
			creationDate, err := time.Parse(twsTimeFormat, string(k))
			if err != nil {
				return err
			}
			post.creationDate = creationDate
			posts = append(posts, *post)
		}

		return nil
	})

	return posts, err
}

func (db *twsDB) getUserPost(ownerID []byte, key []byte) (post dbPost, err error) {
	err = db.db.View(func(tx *bolt.Tx) error {
		bucket, err := db.getPostsBucket(tx, ownerID, false)
		if err != nil {
			return err
		}

		postJson := bucket.Get(key)
		err = json.Unmarshal(postJson, &post)
		if err != nil {
			return err
		}
		return nil
	})
	return
}


// SyncUser Add user if doesn't exists or update current database data
func (db *twsDB) SyncUser(userData TwsUserData) (TwsUserData, error) {
	userResultData := userData
	err := db.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("Users"))
		var dbUser dbUserData
		userDBData := bucket.Get([]byte(userData.UserID))
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
		return bucket.Put([]byte(userData.UserID), dbByteData)
	})
	if err != nil {
		return TwsUserData{}, err
	}

	return userResultData, nil
}

func setUserPrivilege(db *bolt.DB, userId []byte, userRight UserRight) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("Users"))
		user := bucket.Get(userId)
		if user == nil {
			return fmt.Errorf("Coudn't find user with UserID %v\n", userId)
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

func wipeAllUsers(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte("Users"))
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