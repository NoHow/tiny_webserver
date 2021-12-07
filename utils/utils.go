package utils

import (
	"encoding/binary"
	"math/rand"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63() % int64(len(letterBytes))]
	}

	return string(b)
}

func FindString(slice []string, elemToFind string) (int, string) {
	for i, v := range slice {
		if v == elemToFind {
			return i, v
		}
	}

	return -1, ""
}

func FindInt(s []int, elemToFind int) (int, int) {
	for i, v := range s {
		if v == elemToFind {
			return i, v
		}
	}

	return -1, 0
}

func Itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}