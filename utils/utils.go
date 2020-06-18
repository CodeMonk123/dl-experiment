package utils

import (
	"math/rand"
	"time"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

var random = rand.New(rand.NewSource(time.Now().Unix()))

func GenerateUniqueString(length int) string {
	id := make([]byte, length)
	for i := 0; i < length; i++ {
		id[i] = alphabet[random.Intn(len(alphabet))]
	}
	return string(id)
}
