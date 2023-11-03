package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"testing"

	"github.com/eensymachines/tgramscraper/models"
	"github.com/stretchr/testify/assert"
)

func TestBotGetUpdates(t *testing.T) {
	url := fmt.Sprintf("%s%s/getUpdates", BASEURL, BOTTOK)
	resp, err := http.Get(url)
	assert.Nil(t, err, "Unexpected err when sending the http request")
	assert.Equal(t, resp.StatusCode, 200, "Unexpected status code for getUpdates request")
	byt, err := io.ReadAll(resp.Body)
	assert.Nil(t, err, "Unexpected err when reading the payload from getUpdates")
	Updt := models.UpdateResponse{}
	err = json.Unmarshal(byt, &Updt)
	assert.Nil(t, err, "Unexpected err unmarshaling the payload")
	t.Log("lets see the output to the body")
	t.Log(Updt)
	n := new(big.Int)
	n.SetString(Updt.Result[len(Updt.Result)-1].UpdtID.String(), 10)
	t.Log(n.String())
}

func TestBasicHTTPEndpoint(t *testing.T) {
	url := fmt.Sprintf("http://localhost:8080/bots/%s/scrape/0", BOTCHATID)
	resp, _ := http.Post(url, "application/json", nil)
	assert.Equal(t, 200, resp.StatusCode, "Unexpected code when url is valid")
	notOkURLs := []string{
		fmt.Sprintf("http://localhost:8080/bots/%s/scrape/0", "InvalidChatID"),
		fmt.Sprintf("http://localhost:8080/bots/%s/scrape/0", " "),
		// fmt.Sprintf("http://localhost:8080/bots/%s/scrape/0", "%$^&$^%$"),
		// special characters in the url would mean the request does not go thru at all
		fmt.Sprintf("http://localhost:8080/bots/%s/scrape/0", ""),
	}
	for _, url := range notOkURLs {
		resp, err := http.Post(url, "application/json", nil)
		assert.Nil(t, err, "Unexpected err when post request")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Unepxected status code when botchat id is invalid")
	}
	// then testing for invalid offsets
	notOkURLs = []string{
		fmt.Sprintf("http://localhost:8080/bots/5157350442/scrape/%s", "yuy5345"),
		fmt.Sprintf("http://localhost:8080/bots/5157350442/scrape/%s", "rtytry"),
		fmt.Sprintf("http://localhost:8080/bots/5157350442/scrape/%s", " "),
	}
	for _, url := range notOkURLs {
		resp, err := http.Post(url, "application/json", nil)
		assert.Nil(t, err, "Unexpected err when post request")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Unepxected status code when botchat id is invalid")
	}
}
