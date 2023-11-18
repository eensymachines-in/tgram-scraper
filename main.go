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
	"time"

	"github.com/eensymachines/tgramscraper/brokers"
	"github.com/eensymachines/tgramscraper/models"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

var (
	FVerbose, FLogF, FSeed bool
	logFile                string
	RabbitConn             *amqp.Connection // app wide connection used to broadcast the messages received from telegram server
	BotsRegistry           models.TokenRegistry
)

// details of the bot are from secret configurations
// token for the bot cannot be exposed
const (
	// Details on botmincock.. since that bot isnt functional for now
	BASEURL = "https://api.telegram.org/bot"
	// following 2 should be received from secrets
	BOTTOK    = "6133190482:AAFdMU-49W7t9zDoD5BIkOFmtc-PR7-nBLk"
	BOTCHATID = "6133190482"
	NIRCHATID = "5157350442" // this id is the chat id of the developer

	REQTIMEOUT   = 6 * time.Second
	RABBIT_QUEUE = "tgramscrape_messages"
	// constants below are incase when the environment variables arent loaded
	// IMP: do not use them production
	AMQP_USER   = "guest"
	AMQP_PASSWD = "guest"
	AMQP_SERVER = "localhost:30073" // server address inclusive of te port
)

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

	// ------------- Reading in the environment
	// ------- forming the connection string
	user := os.Getenv("AMQP_USER")
	if user == "" {
		user = AMQP_USER
	}
	passwd := os.Getenv("AMQP_PASSWD")
	if passwd == "" {
		passwd = AMQP_PASSWD
	}
	server := os.Getenv("AMQP_SERVER")
	if server == "" {
		server = AMQP_SERVER
	}

	// Making rabbit mq connection
	// declaring a queue that all the subscribers listen to
	//svc-rabbit:5672
	//localhost:30072
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s/", user, passwd, server))
	if err != nil || conn == nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to make amqp connection to rabbit mq")
		panic(err)
	}
	log.Info("Established connection to rabbitmq broker")

	RabbitConn = conn
	// TODO:
	// this registry can be hydrated from environment / secret files
	// for all the development purposes we have left it hardcoded for now
	BotsRegistry = models.NewSimpleTokenRegistry("6133190482:AAFdMU-49W7t9zDoD5BIkOFmtc-PR7-nBLk")
	log.WithFields(log.Fields{
		"count": BotsRegistry.Count(),
	}).Debug("botsregistry read in")
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
	rabbit := brokers.Broker(&brokers.RabbitMQBroker{Conn: RabbitConn, QName: RABBIT_QUEUE})
	err := rabbit.(brokers.QueuedBroker).DeclareQueue(RABBIT_QUEUE)
	if err != nil {
		log.WithFields(log.Fields{
			"err-msg": err,
		}).Error("failed to initiate a queue on the broker")
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"err": "Received message from Telegram server, but could not broker",
		})
		return
	}
	// Response writer
	scraper := models.Scraper(&models.TelegramScraper{UID: ctx.Param("botid"), Offset: ctx.Param("updtid"), Registry: BotsRegistry, Broker: rabbit})
	resp, err := scraper.Scrape(REQTIMEOUT)
	if err != nil {
		log.WithFields(log.Fields{
			"botid":          ctx.Param("botid"),
			"offset":         ctx.Param("updtid"),
			"count_reg_bots": BotsRegistry.Count(),
			"broker_nil":     fmt.Sprintf("%t", BotsRegistry.Count() > 0),
		}).Errorf("failed to scrape/TelegramScraper: %s", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"err": fmt.Sprintf("bot ID unregistered or is invalid: %s", ctx.Param("botid")),
		})
		return
	}
	ctx.AbortWithStatusJSON(http.StatusOK, resp)
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
	r.POST("/bots/:botid/scrape/:updtid", HndlScrapeTrigger)

	log.Fatal(r.Run(":8080"))
}
