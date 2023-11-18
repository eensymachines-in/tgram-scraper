package models

/* ---------------------------------------
For accessing the bots via the http APIs a unique generated secret token is what is required
Tokens are a combination of the uid and the token itself
Register any bot by sending in the token of the bot. Registry would store the same in retreivable format
Scraping is bot specific - ID of the bot is specified in the trigger url to indicate the messages of the bot to be accecssed.
--------------------------------------- */
import (
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	tokenRegx = regexp.MustCompile(`^[0-9]{10}:[\w\W\d]{6}-[\w\W\d]{19}-[\w\W\d]{3}-[\w\W\d]{4}$`)
)

type TokenRegistry interface {
	Find(uid string) (string, bool)
	Count() int
}

// For the uid of the bot this can store the token of the bot
type SimpleTokenRegistry struct {
	Data map[string]string // key value pairs for token and uid of the bots
}

// NewSimpleTokenRegistry : its possible to get the uid of the bot from the token supplied
// here simple registry will just make the data from the given tokens
func NewSimpleTokenRegistry(tokens ...string) TokenRegistry {
	reg := &SimpleTokenRegistry{Data: map[string]string{}}
	for i, tok := range tokens {
		if tokenRegx.MatchString(tok) {
			result := strings.Split(tok, ":")
			if len(result) <= 0 {
				log.WithFields(log.Fields{
					"index": i,
					"data":  tok,
				}).Error("failed to parse token for registry")
				continue
			}
			if _, ok := reg.Data[result[0]]; ok {
				log.Warnf("UID already found registered: %s", result[0])
				continue
			}
			reg.Data[result[0]] = tok
		} else {
			// if the token does no conform to the regex of the typical bot token
			log.Errorf("Failed to parse token to registry : %s", tok)
			continue
		}
	}
	return reg
}

func (str *SimpleTokenRegistry) Count() int {
	return len(str.Data)
}

// Find : for a given uid of the bot ths can give us the registered token of the bot
// note: the uid is not the same as the chat id
func (str *SimpleTokenRegistry) Find(uid string) (string, bool) {
	for k, v := range str.Data {
		if k == uid {
			return v, true
		}
	}
	return "", false
}
