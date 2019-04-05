package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
)

func getSha(data *PostData) string {
	sha := sha256.Sum256(data.PythonScript)
	postHash := fmt.Sprintf("%x", sha)
	return postHash
}

func getPostData(c *gin.Context) (*PostData, error) {
	postData := new(PostData)
	rb, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	postData.PythonScript = rb
	return postData, nil
}
