package utils

import "math/rand"

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