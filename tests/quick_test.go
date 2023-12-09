package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/eensymachines/tgramscraper/brokers"
	"github.com/eensymachines/tgramscraper/scrapers"
	"github.com/stretchr/testify/assert"
)

// triggerScrape : call this on intervals to trogger the u-service to scrape the updates from telegram server. Sends a post request
//
//	offset 		: this is the offset by which the total updates are filtered
//	error incase the http request isn't a success response
func triggerScrape(offset, url string) (*scrapers.ScrapeResult, error) {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/bots/6133190482/scrape/%s", url, offset), nil)
	// req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:30001/bots/6133190482/scrape/%s", offset), nil)
	if err != nil {
		return nil, err
	}
	cl := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := cl.Do(req)
	if resp.StatusCode != http.StatusOK || err != nil {
		return nil, err
	}
	byt, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &scrapers.ScrapeResult{}
	if err := json.Unmarshal(byt, result); err != nil {
		return nil, err
	}
	return result, nil

}

// TestBlackBox : provides a single black box test for the entire service as a package.
// After running the service, you need to start this test and then type in messages in the targetted bot chat.
// All the messages you type will would make a trip from telegram server to rabbitmq and then on the terminal of this test
// run the microservice NOT under a container, or a pod but as a local go lang application
// NOTE: the environment variables need to be set as well as the secret files need to be enabled and populated on the host.
// NOTE: for now the secrets are made hard coded
func TestBlackBox(t *testing.T) {
	// setup environment variables  ..
	t.Setenv("SCRAPE_SERVER", "http://localhost:8080")
	t.Setenv("AMQP_SERVER", "localhost:30073")
	t.Setenv("BASEURL", "https://api.telegram.org")
	t.Setenv("NIRCHATID", "5157350442")

	connResult, err := brokers.RabbitConnDial("guest", "guest", os.Getenv("AMQP_SERVER"))
	assert.Nil(t, err, "unexpected error when setting up the test: %s", err)
	assert.NotNil(t, connResult, "Unexpected nil conn result")
	err = connResult.BindAQueue("test.listener", "amq.topic", "6133190482.updates")
	assert.Nil(t, err, "unexpected error when binding queue to rabbit exchange")
	listen, err := connResult.ListenOnQueue("test.listener")
	assert.Nil(t, err, "Unexpected error when setting up the listening channel")
	blockUntil := make(chan bool)
	// Here after every interval we would trigger the service to scrape the telegram server for the updates

	go func() {
		offset := "0"
		counter := 0
		defer close(blockUntil)
		for {
			counter += 1
			if counter <= 50 {
				select {
				case <-time.After(3 * time.Second):
					res, err := triggerScrape(offset, os.Getenv("SCRAPE_SERVER"))
					if err != nil {
						t.Error(err)
					} else {
						offset = res.NextUpdateOffset
					}
				case update := <-listen:
					t.Logf("received :%s", string(update.Body))
				}
			} else {
				return
			}
		}

	}()
	<-blockUntil
}
