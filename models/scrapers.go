package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/eensymachines/tgramscraper/brokers"
)

// TelegramScraper : agent to call the telegram server to get updates with/without offset
// From the registry can get the details of the bot - token, tokens are necessary when getting the updates
// Broker helps sending the message over to the message broker
type TelegramScraper struct {
	UID      string
	Offset   string
	Registry TokenRegistry
	Broker   brokers.Broker
	// Writer   ResponseWriter
}

// Scrape : getupdates > send the message over to the broker >return reponse result (sumamry of the update)
func (ts *TelegramScraper) Scrape(reqTimeOut time.Duration) (map[string]interface{}, error) {
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
			Timeout: reqTimeOut,
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send http reuest to Telegram server %s", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error response from telegram server %d", resp.StatusCode)
		}

		// statusok , reading the response body
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			// TODO: fix ahead from here
			return nil, fmt.Errorf("error reading the response body: %s", err)
		}
		updtResp := UpdateResponse{}
		err = json.Unmarshal(byt, &updtResp)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal update response from server %s", err)
		}
		updtResp.BotID = ts.UID // bot id is nowhere to be found in the update - hence attaching the same
		byt, err = json.Marshal(updtResp)
		if err != nil {
			return nil, fmt.Errorf("failed to read response from Telegram server %s", err)
		}
		if len(updtResp.Result) > 0 {
			// NOTE: publishing on the broker only if there are message results in the update
			if err := ts.Broker.Publish(byt); err != nil {
				return nil, err
			}
		}
		return map[string]interface{}{
			"totalUpdates": len(updtResp.Result),
			"updateOffset": func() string {
				// gets the offset required for the next update
				// offsets since they are large numbers,storethem as string
				// for getting next offset, temp conversion  to big int and then back to string for result
				n := new(big.Int)
				if len(updtResp.Result) > 0 {
					val, _ := n.SetString(updtResp.Result[len(updtResp.Result)-1].UpdtID.String(), 10)
					val = n.Add(val, big.NewInt(1))
					return val.String()
				}
				return n.String()
			}(),
			"allMessages": func() []string { // collects texts of all the messages
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

type Scraper interface {
	Scrape(time.Duration) (map[string]interface{}, error)
}
