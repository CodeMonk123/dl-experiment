package utils

import (
	"fmt"
	"net/http"
)

func TunerIsAvailable(ip string) bool {
	testUrl := fmt.Sprintf("%s:%s", ip, ":8001/api/v1/tuner/hello")
	if _, err := http.Get(testUrl); err == nil {
		return true
	}
	return false
}
