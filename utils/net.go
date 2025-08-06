package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func Download(url string, filepath string) (err error) {
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return
	}
	defer f.Close()

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status code error: %d", resp.StatusCode)
		return
	}
	_, err = io.Copy(f, resp.Body)
	return
}
