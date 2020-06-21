package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func TunerIsAvailable(ip string) error {
	testUrl := fmt.Sprintf("http://%s:%s", ip, "8001/api/v1/tuner/hello")
	_, err := http.Get(testUrl)
	return err
}

func GetNewParamFromTuner(ip string, index int) (string, error) {
	url := fmt.Sprintf("http://%s:%s", ip, "8001/api/v1/tuner/new_param")
	body, _ := json.Marshal(map[string]int{"index": index})
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	content, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	param := string(content)
	return param, nil
}
