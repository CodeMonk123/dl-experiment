package utils

import (
	"fmt"
	"net/http"
)

func TunerIsAvailable(ip string) error {
	testUrl := fmt.Sprintf("%s:%s", ip, ":8001/api/v1/tuner/hello")
	_, err := http.Get(testUrl)
	return err
}
