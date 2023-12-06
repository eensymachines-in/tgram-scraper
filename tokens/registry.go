// u-services are bot agnostic, hence need maintenance of token registry of each valid bot

// For accessing the bots via the http APIs a unique generated secret token is what is required
// Tokens are a combination of the uid and the token itself
// Register any bot by sending in the token of the bot. Registry would store the same in retreivable format
// Scraping is bot specific - ID of the bot is specified in the trigger url to indicate the messages of the bot to be accecssed.
package tokens

import (
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	// tokenRegx is the pattern of the string for which the telegram bot token is valid
	// tokenRegx = regexp.MustCompile(`^[0-9]{10}:[\w\W\d]{6}-[\w\W\d]{19}-[\w\W\d]{3}-[\w\W\d]{4}$`)
	tokenRegx = regexp.MustCompile(`^[0-9]{10}:[\w\W\d_-]{35}$`)
)

// Generic public interface for accessing any type of token registry.
// TokenRegistry is used to find and measure the registered bots by their tokens.
// Provides a common interface for any type of registry.
type TokenRegistry interface {
	Find(uid string) (string, bool) // given the uid of the bot gets the token
	Count() int                     // counts the number of registered bots
}

// For the uid of the bot this can store the token of the bot
type SimpleTokenRegistry struct {
	Data map[string]string // key value pairs for token and uid of the bots
}
// 5234189659:AAFhRYn_Rmg4EvAtC6nkraPZjgttiBLWFdg
// NewSimpleTokenRegistry creates a SimpleTokenRegistry object over TokeneRegistry interface.
// From the given string token, this can extract the token and uid of the bot.
// Incase the token is invalid, it'd silently continue without adding the registration, but will log the error.
// It isnt possible to add tokens to already existing registry - reason being registries are designed to be populated only once at the the start of the application run.
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

// Gets the count of the registered bot tokens.
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
