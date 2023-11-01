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
