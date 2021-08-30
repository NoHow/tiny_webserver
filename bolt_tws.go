package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"os"
	"strings"
)

type twsDB struct {
	db *bolt.DB
}

type dbUserData struct {
	AvatarUrl string
	AdminRight UserRight
}

func InitDB() {
	db, err := bolt.Open("data/tws.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("PagesData"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("Users"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

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
				fmt.Printf("User id: %s, info: %v\n", key, userData)
			} else {
				fmt.Printf("Something wrong with user data from the user with id - %s\n", key)
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