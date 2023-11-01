package main

/* ========================
u-service based on Go gin to provide an http endpoint that can be triggered when applications need to scrape Telegram bot messages. 
The endpoint is agnostic to any bot. When identified with bot ID, or the chat ID for the bot this can then check the bot for messages.
author 		:kneerunjun@gmail.com
date		:01-NOV-2023
===========================*/
import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"regexp"

	"github.com/eensymachines/tgramscraper/models"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var (
	FVerbose, FLogF, FSeed bool
	logFile                string
)
// details of the bot are from secret configurations 
// token for the bot cannot be exposed 
const (
	// Details on botrunjun
	BASEURL   = "https://api.telegram.org/bot"
	BOTTOK    = "5234189659:AAFhRYn_Rmg4EvAtC6nkraPZjgttiBLWFdg"
	BOTCHATID = 5157350442
)

func init() {
	// Setting up log configuration for the api
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: false,
		ForceColors:   true,
	})
	log.SetReportCaller(false)
	// By default the log output is stdout and the level is info
	log.SetOutput(os.Stdout)     // FLogF will set it main, but dfault is stdout
	log.SetLevel(log.DebugLevel) // default level info debug but FVerbose will set it main
	logFile = os.Getenv("LOGF")
}

func getBotTokFromID(chatid string) string {
	// Ideally speaking this shall come from secrets and configuration
	// for each of the bot id - or the botchat id the token is to be retrieved from secrets file
	// NOTE: for now we just send the hard coded value back
	return BOTTOK
}

// getUpdatesURL : Telegram server needs appropriate url that can be queried for offset
func getUpdatesURL(baseurl string, bottok string, offset string) string {
	return fmt.Sprintf("%s%s/getUpdates?offset=%s", baseurl, bottok, offset)
}

// HndlScrapeTrigger : function to handle the endpoint hit
func HndlScrapeTrigger(ctx *gin.Context) {
	// -------- reading the url params
	// IDs - botid, updateid are better off in strin format until any mathematical opertation
	// all numerical ids are checked for input and sends back a bad request code whebn its not
	// ---------
	rgx := regexp.MustCompile(`^[0-9]+$`)
	if !rgx.MatchString(ctx.Param("botid")) {
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
	if !rgx.MatchString(ctx.Param("updtid")) {
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
	// -------- Making http request to Telegram server
	// 	- uses the base url common for all the requests
	// 	- bot token to identify the bot uniquely
	//  - offset id from the url
	// --------
	resp, err := http.Get(getUpdatesURL(BASEURL, getBotTokFromID(ctx.Param("botid")), ctx.Param("updtid")))
	if err != nil { // typically when no internet connection
		errMsg := fmt.Errorf("failed to make request to telegram server")
		log.WithFields(log.Fields{
			"err-msg": errMsg,
		}).Error(errMsg)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"err": errMsg,
		})
		return
	}
	if resp.StatusCode == 200 {
		//  read the update and send it across as a response
		// moving ahead we would post it to a cache for other services to pick up
		// also update the last update id for the offset for the next trigger
		updt := models.UpdateResponse{}
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"err-msg": err,
			}).Error("failed to read response body from telegram server")
			ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
				"err": "Error reading the response from server",
			})
			return
		}
		err = json.Unmarshal(byt, &updt)
		if err != nil {
			log.WithFields(log.Fields{
				"err-msg": err,
			}).Error("failed to unmarshal response body from telegram server")
			ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
				"err": "Error reading the response from server",
			})
			return
		}
		// The caller of this endpoint should know whats the next updateID to call
		allUpdates := []string{}
		for _, r := range updt.Result {
			allUpdates = append(allUpdates, r.Message.Text)
		}
		ctx.AbortWithStatusJSON(http.StatusOK, gin.H{
			"updateOffset": func() *big.Int {
				n := new(big.Int)
				if len(updt.Result) > 0 {
					val, _ := n.SetString(updt.Result[len(updt.Result)-1].UpdtID.String(), 10)
					val = n.Add(val, big.NewInt(1))
					return val
				}
				return n
			}(), // last result read add one to the update ID
			"totalUpdates": len(updt.Result),
			"allMessages":  allUpdates,
		})
		return
	} else { // failed response code from Telegram server
		errMsg := "error response from telegram server"
		log.WithFields(log.Fields{
			"url":     ctx.Request.URL.String(),
			"code":    resp.StatusCode,
			"err-msg": errMsg,
		}).Error(errMsg)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"err": errMsg,
		})
		return
	}
}
func main() {
	flag.Parse() // command line flags are parsed
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