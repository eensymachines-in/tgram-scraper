package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"

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
	url := fmt.Sprintf("http://localhost:30001/bots/%s/scrape/0", BOTCHATID)
	resp, _ := http.Post(url, "application/json", nil)
	assert.Equal(t, 200, resp.StatusCode, "Unexpected code when url is valid")
	notOkURLs := []string{
		fmt.Sprintf("http://localhost:30001/bots/%s/scrape/0", "InvalidChatID"),
		fmt.Sprintf("http://localhost:30001/bots/%s/scrape/0", " "),
		// fmt.Sprintf("http://localhost:30001/bots/%s/scrape/0", "%$^&$^%$"),
		// special characters in the url would mean the request does not go thru at all
		fmt.Sprintf("http://localhost:30001/bots/%s/scrape/0", ""),
	}
	for _, url := range notOkURLs {
		resp, err := http.Post(url, "application/json", nil)
		assert.Nil(t, err, "Unexpected err when post request")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Unepxected status code when botchat id is invalid")
	}
	// then testing for invalid offsets
	notOkURLs = []string{
		fmt.Sprintf("http://localhost:30001/bots/5157350442/scrape/%s", "yuy5345"),
		fmt.Sprintf("http://localhost:30001/bots/5157350442/scrape/%s", "rtytry"),
		fmt.Sprintf("http://localhost:30001/bots/5157350442/scrape/%s", " "),
	}
	for _, url := range notOkURLs {
		resp, err := http.Post(url, "application/json", nil)
		assert.Nil(t, err, "Unexpected err when post request")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Unepxected status code when botchat id is invalid")
	}
}

func TestCronTriggerOnHTTPEndpoint(t *testing.T) {
	// This runs a continuous loop on sending POST requests to this endpoint to see if we can get all the updates as expected
	// Run this test and then head over to Telegram chat and send messages
	// All those messages would be available here
	offset := big.NewInt(0)
	for {
		t.Log("Now triggering a scrape")

		url := fmt.Sprintf("http://localhost:30001/bots/%s/scrape/%d", BOTCHATID, offset)
		resp, err := http.Post(url, "application/json", nil)
		
		if resp != nil && err == nil {
			assert.Equal(t, 200, resp.StatusCode, "Unexpected error code when url is valid")
			// Recalculating the offset from the payload received
			res := map[string]interface{}{}
			byt, err := io.ReadAll(resp.Body)
			assert.Nil(t, err, fmt.Sprintf("Unexpected error when reading the body %s", err))
			err = json.Unmarshal(byt, &res)
			assert.Nil(t, err, fmt.Sprintf("Unexpected error when unmarshalling the body %s", err))
			t.Logf("Updates received %v", res["totalUpdates"])
			messages, _ := res["allMessages"].([]interface{})
			for _, v := range messages {
				t.Log(v)
			}
			val, _ := res["updateOffset"].(string)
			offset, _ = offset.SetString(val, 10)
		}

		<-time.After(5 * time.Second)
	}
}
