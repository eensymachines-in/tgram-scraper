package main

/* ========================
u-service based on Go gin to provide an http endpoint that can be triggered when applications need to scrape Telegram bot messages.
The endpoint is agnostic to any bot. When identified with bot ID, or the chat ID for the bot this can then check the bot for messages.
This service is entirely stateless -
- Agnostic of the bot id and token. when received as a param this can send the getUpdates request to any bot
- Does not store the last queried update id / offset - thats the responsibility of the caller
-
author 		:kneerunjun@gmail.com
date		:01-NOV-2023
===========================*/
import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/eensymachines/tgramscraper/brokers"
	"github.com/eensymachines/tgramscraper/scrapers"
	"github.com/eensymachines/tgramscraper/tokens"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

var (
	FVerbose, FLogF, FSeed bool
	logFile                string
	RabbitConn             *amqp.Connection // app wide connection used to broadcast the messages received from telegram server
	BotsRegistry           tokens.TokenRegistry
)

// details of the bot are from secret configurations
// token for the bot cannot be exposed
const (
	// Details on botmincock.. since that bot isnt functional for now

	// following 2 should be received from secrets
	BOTTOK    = "6133190482:AAFdMU-49W7t9zDoD5BIkOFmtc-PR7-nBLk"
	BOTCHATID = "6133190482"
	// this id is the chat id of the developer
	REQTIMEOUT   = 6 * time.Second
	RABBIT_QUEUE = "tgramscrape_messages"
)

var (
	// Below are the values that are used for local testing
	// IMP: not to use in production
	AMQP_USER   = "guest"
	AMQP_PASSWD = "guest"
	AMQP_SERVER = "localhost:30073" // server address inclusive of te port

	BASEURL      = "https://api.telegram.org/bot"
	NIRCHATID    = "5157350442"
	SECRET_MOUNT = "/run/secrets/vol-tgramsecrets/" // this is where the secrets are mounted
	TGRAM_SECRET = "bottoks"                        // when configured on kubernetes this is the name of the secret you want to access
)

// loadBotTokenSecrets : from the mounted secrets this can split get all the distinct tokens
func loadBotTokenSecrets() ([]string, error) {
	filepath := fmt.Sprintf("%s%s", SECRET_MOUNT, TGRAM_SECRET)
	byt, err := os.ReadFile(filepath)
	if err != nil || byt == nil {
		// Unable to read file - secrets not loaded
		return []string{}, fmt.Errorf("error reading the bottokens from secrets %s", err)
	}
	return strings.Split(string(byt), " "), nil
}

func init() {
	// Setting up log configuration for the api
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: false,
		ForceColors:   true,
		PadLevelText:  true,
	})
	log.SetReportCaller(false)
	// By default the log output is stdout and the level is info
	log.SetOutput(os.Stdout)     // FLogF will set it main, but dfault is stdout
	log.SetLevel(log.DebugLevel) // default level info debug but FVerbose will set it main
	logFile = os.Getenv("LOGF")

	user := os.Getenv("AMQP_USER")
	if user != "" {
		AMQP_USER = user
	}
	passwd := os.Getenv("AMQP_PASSWD")
	if passwd != "" {
		AMQP_PASSWD = passwd
	}
	server := os.Getenv("AMQP_SERVER")
	if server != "" {
		AMQP_SERVER = server
	}

	log.WithFields(log.Fields{
		"user":   user,
		"server": server,
	}).Debug("Read in environment variables")

	toks, err := loadBotTokenSecrets()
	if err != nil {
		log.Panic(err)
	}
	BotsRegistry = tokens.NewSimpleTokenRegistry(toks...)
	log.WithFields(log.Fields{
		"count": BotsRegistry.Count(),
	}).Debug("botsregistry read in")
}

// HndlRabbitPublish : message received in context from the previous handlers is published to the rabbit broker
func HndlRabbitPublish(ctx *gin.Context) {
	conn, err := brokers.RabbitConnDial(AMQP_USER, AMQP_PASSWD, AMQP_SERVER)
	if err != nil || conn == nil {
		log.WithFields(log.Fields{
			"user":   AMQP_USER,
			"server": AMQP_SERVER,
			"err":    err,
		}).Error("failed HndlRabbitPublish: unsuccessful rabbit dial connection")
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"err": "One ",
		})
		return
	}

	defer conn.CloseConn()
	val, ok := ctx.Get("scrape_result")
	if !ok {
		log.WithFields(log.Fields{
			"scrape_result": val,
		}).Error("failed HndlRabbitPublish: invalid or empty scrape result")
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"err": "One ",
		})
		return
	}
	botUpdate, ok := val.(*scrapers.ScrapeResult)
	if !ok || botUpdate == nil {
		log.Error("failed HndlRabbitPublish: Invalid type of scrape result, expected *scrapers.ScrapeResult")
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	publishTopic := fmt.Sprintf("%s.updates", botUpdate.ForBot)
	for _, updt := range botUpdate.AllMessages {
		// NOTE: the broker gets each message published independently, not as an slice
		// incase there arent any results, no publications
		conn.BindAQueue("test.listener", "amq.topic", publishTopic)
		err = conn.Publish([]byte(updt), "amq.topic", publishTopic)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("failed HndlRabbitPublish: failed to publish to rabbit broker")
			ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
				"err": "Received updates, but failed to publish",
			})
			return
		}
	}
	ctx.AbortWithStatusJSON(http.StatusOK, botUpdate)
}

func HndlScrapeTrigger(ctx *gin.Context) {
	rgx := regexp.MustCompile(`^[0-9]+$`)     // url params checked
	if !rgx.MatchString(ctx.Param("botid")) { // always numerical id
		errMsg := fmt.Errorf("invalid bot chat id in url, check & send again")
		log.WithFields(log.Fields{
			"err-msg": errMsg,
			"botid":   ctx.Param("botid"),
		}).Error(errMsg)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"err": errMsg,
		})
		return
	}
	if !rgx.MatchString(ctx.Param("updtid")) { // validating updtid
		errMsg := fmt.Errorf("invalid bot update offset in url, check & send again")
		log.WithFields(log.Fields{
			"err-msg":   errMsg,
			"update-id": ctx.Param("updtid"),
		}).Error(errMsg)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"err": errMsg,
		})
		return
	}
	// Preparing the broker to be consumed by the scraper
	// making a new rabbit mq broker
	// TODO: access rabbit broker and post the mesasge

	// Response writer
	scraper := scrapers.Scraper(&scrapers.TelegramScraper{UID: ctx.Param("botid"), Offset: ctx.Param("updtid"), Registry: BotsRegistry})
	resp, err := scraper.Scrape(scrapers.ScrapeConfig{RequestTimeout: 6 * time.Second})
	if err != nil {
		log.WithFields(log.Fields{
			"botid":          ctx.Param("botid"),
			"offset":         ctx.Param("updtid"),
			"count_reg_bots": BotsRegistry.Count(),
			"broker_nil":     fmt.Sprintf("%t", BotsRegistry.Count() > 0),
		}).Errorf("failed to scrape/TelegramScraper: %s", err)
	}
	ctx.Set("scrape_result", resp) // downstreaming processing of the scrape
	ctx.Next()
}

func main() {
	defer RabbitConn.Close() // cleaning up the connection when not required
	flag.Parse()             // command line flags are parsed
	log.WithFields(log.Fields{
		"verbose": FVerbose,
		"flog":    FLogF,
		"seed":    FSeed,
	}).Info("Log configuration..")
	if FVerbose {
		log.SetLevel(log.DebugLevel)
	}
	if FLogF {
		lf, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Failed to connect to log file, kindly check the privileges")
		} else {
			log.Infof("Check log file for entries @ %s", logFile)
			log.SetOutput(lf)
		}
	}
	log.Info("Now starting the telegram scraper microservice")
	gin.SetMode(gin.DebugMode)
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"app":    "Telegram scraper",
			"author": "kneerunjun@gmail.com",
			"date":   "November 2023",
			"msg":    "If you are able to see this, you know the telegram scraper is working fine",
		})
	})
	r.POST("/bots/:botid/scrape/:updtid", HndlScrapeTrigger, HndlRabbitPublish)

	log.Fatal(r.Run(":8080"))
}
