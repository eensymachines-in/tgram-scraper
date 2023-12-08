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
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"reflect"
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

var (
	AMQP_USER, AMQP_PASSWD, AMQP_SERVER string
	BASEURL, NIRCHATID                  string

	SECRET_MOUNT = "/run/secrets/vol-tgramsecrets/" // this is where the secrets are mounted
	TGRAM_SECRET = "bottoks"                        // when configured on kubernetes this is the name of the secret you want to access

	AMQP_SECRET_MOUNT = "/run/secrets/vol-amqpsecrets/"
	AMQP_SECRET       = "user password"
)

// loadBotTokenSecrets : from the mounted secrets this can split get all the distinct tokens
func loadBotTokenSecrets() ([]string, error) {
	filepath := fmt.Sprintf("%s%s", SECRET_MOUNT, TGRAM_SECRET)
	byt, err := os.ReadFile(filepath)
	if err != nil || byt == nil {
		// Unable to read file - secrets not loaded
		return []string{}, fmt.Errorf("error reading the bottokens from secrets %s", err)
	}
	if bytes.HasSuffix(byt, []byte("\n")) { //often file read in will have this as a suffix
		byt, _ = bytes.CutSuffix(byt, []byte("\n"))
	}
	return strings.Split(string(byt), " "), nil
}
func loadAMQPCredentials() (string, string, error) {
	var user, pass string
	secrets := strings.Split(AMQP_SECRET, " ")
	for _, s := range secrets {
		filepath := fmt.Sprintf("%s%s", AMQP_SECRET_MOUNT, s)
		byt, err := os.ReadFile(filepath)
		if err != nil || byt == nil {
			// Unable to read file - secrets not loaded
			return "", "", fmt.Errorf("error reading the amqp secrets %s", err)
		}
		if bytes.HasSuffix(byt, []byte("\n")) { //often file read in will have this as a suffix
			byt, _ = bytes.CutSuffix(byt, []byte("\n"))
		}
		if s == "user" {
			user = string(byt)
		}
		if s == "password" {
			pass = string(byt)
		}
	}
	return user, pass, nil
}
func init() {
	/* -------------
	Setting up log configuration for the api
	----------------*/
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

	/* -------------
	Environment variables || default values
	----------------*/
	AMQP_SERVER = os.Getenv("AMQP_SERVER")
	NIRCHATID = os.Getenv("NIRCHATID")
	BASEURL = os.Getenv("BASEURL")
	if AMQP_SERVER == "" || NIRCHATID == "" || BASEURL == "" {
		log.WithFields(log.Fields{
			"server":    AMQP_SERVER,
			"nirchatid": NIRCHATID,
			"baseurl":   BASEURL,
		}).Panic("failed to load one or more environment veairbles")
	}
	log.Debug("Environment vars loadeed..")

	var err error
	AMQP_USER, AMQP_PASSWD, err = loadAMQPCredentials()
	if err != nil {
		log.WithFields(log.Fields{
			"user":     AMQP_USER,
			"password": AMQP_PASSWD,
			"err":      err,
		}).Panic("failed to read secret amqo credentials")
	}
	log.WithFields(log.Fields{
		"user":     AMQP_USER,
		"password": AMQP_PASSWD,
	}).Debug("AMQP credentials read in..")

	/* -------------
	Loading telegram bot secrets
	------------- */
	toks, err := loadBotTokenSecrets()
	if err != nil {
		log.Panic(err)
	}
	for _, t := range toks {
		log.WithFields(log.Fields{
			"tok": t,
		}).Debug("token")
	}
	BotsRegistry = tokens.NewSimpleTokenRegistry(toks...)
	log.WithFields(log.Fields{
		"count": BotsRegistry.Count(),
	}).Debug("botsregistry read in")

	// Testing amqp connection , and aborting early
	_, err = brokers.RabbitConnDial(AMQP_USER, AMQP_PASSWD, AMQP_SERVER)
	if err != nil {
		log.WithFields(log.Fields{
			"user":   AMQP_USER,
			"server": AMQP_SERVER,
			"err":    err,
		}).Panic("failed to connect to AMQP server")
	}
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
	log.Debug("rabbit connected..")
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
		log.WithFields(log.Fields{
			"update_type": reflect.TypeOf(botUpdate).String(),
		}).Error("failed HndlRabbitPublish: Invalid type of scrape result, expected *scrapers.ScrapeResult")
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	publishTopic := fmt.Sprintf("%s.updates", botUpdate.ForBot) // fixing the topic under which the message is published.
	for _, updt := range botUpdate.AllMessages {
		// NOTE: the broker gets each message published independently, not as an slice
		// incase there arent any results, no publications
		conn.BindAQueue("test.listener", "amq.topic", publishTopic) // this is only for testing purposes
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
	log.WithFields(log.Fields{
		"count": resp.UpdateCount,
	}).Debug("received updates from telegram server")
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
