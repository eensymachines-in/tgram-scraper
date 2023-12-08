// Are services that can query telegram server when triggered

// Scrapers are implementations of services that can fetch updates from chat servers
package scrapers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/eensymachines/tgramscraper/models"
	"github.com/eensymachines/tgramscraper/tokens"
	log "github.com/sirupsen/logrus"
)

// ScrapeConfig extensible configuration object when scraping
type ScrapeConfig struct {
	RequestTimeout time.Duration // scrape requests refer to http requests made, timeout refers to the same
}

type Scraper interface {
	Scrape(c ScrapeConfig) (*ScrapeResult, error)
}

// TelegramScraper : agent to call the telegram server to get updates with/without offset
// From the registry can get the details of the bot - token, tokens are necessary when getting the updates
// Broker helps sending the message over to the message broker
type TelegramScraper struct {
	UID      string
	Offset   string
	Registry tokens.TokenRegistry
}

// ScrapeResult is the return result after Scrape is called.
type ScrapeResult struct {
	UpdateCount      int      `json:"update_count"` // count of distinct updatess
	NextUpdateOffset string   `json:"offset"`       // for the subsequent request this is used as the offset for getting the updates, large number
	AllMessages      []string `json:"all_messages"` // text messages in each of the updates
	ForBot           string   `json:"for_bot"`      // id of the bot for which this result is relevant, each bot has an id
}

// Scrape : getupdates > send the message over to the broker >return reponse result (sumamry of the update)
func (ts *TelegramScraper) Scrape(c ScrapeConfig) (*ScrapeResult, error) {
	// TODO: finding from the registry shouldnt be the responsibility of the scrapper
	// Need tomove the same from here
	botTok, ok := ts.Registry.Find(ts.UID)
	if !ok {
		// unregistered bot token
		return nil, fmt.Errorf("invalid bot ID, no token found registered against it %s", ts.UID)
	}
	if botTok != "" {
		url := func(tok string) string {
			n := new(big.Int)
			val, _ := n.SetString(ts.Offset, 10)
			if val != big.NewInt(0) {
				return fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%s", botTok, ts.Offset)
			} else {
				// Offset specified when 0 would lead to downloading all the updates that have been previously downloaded
				// Not sending offset param will then get no updates if the updates have been already fetched.
				return fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", botTok)
			}
		}(botTok)
		req, _ := http.NewRequest("GET", url, bytes.NewBuffer([]byte("")))
		client := &http.Client{
			Timeout: c.RequestTimeout,
		}
		resp, err := client.Do(req)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Debug("Scrape: error making the http request, check internet connection")
			return nil, fmt.Errorf("failed to send http reuest to Telegram server %s", err)
		}
		if resp.StatusCode != http.StatusOK {
			log.WithFields(log.Fields{
				"status_code": resp.StatusCode,
			}).Debug("Scrape: Http status code from the telegram server is unfavorable")
			return nil, fmt.Errorf("error response from telegram server %d", resp.StatusCode)
		}

		// statusok , reading the response body
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Debug("Scrape: Error reading response payload from telegram server")
			return nil, fmt.Errorf("error reading the response body: %s", err)
		}
		updtResp := models.UpdateResponse{}
		err = json.Unmarshal(byt, &updtResp)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Debug("Scrape: Error unmarshaling response payload from telegram server")
			return nil, fmt.Errorf("failed to unmarshal update response from server %s", err)
		}
		updtResp.BotID = ts.UID // bot id is nowhere to be found in the update - hence attaching the same
		return &ScrapeResult{
			UpdateCount: len(updtResp.Result),
			NextUpdateOffset: func() string {
				n := new(big.Int)
				if len(updtResp.Result) > 0 {
					val, _ := n.SetString(updtResp.Result[len(updtResp.Result)-1].UpdtID.String(), 10)
					val = n.Add(val, big.NewInt(1))
					return val.String()
				}
				return n.String()
			}(),
			ForBot: ts.UID,
			AllMessages: func() []string { // collects texts of all the messages
				res := []string{}
				for _, r := range updtResp.Result {
					res = append(res, r.Message.Text)
				}
				return res
			}(),
		}, nil
	}
	return nil, fmt.Errorf("no bot with id %s found registered with us. Only registerd bots can scrape", botTok)
}
