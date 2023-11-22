package scrapers

import (
	"testing"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

// https://youtu.be/pAXp6o-zWS4
func TestBasicAmqp(t *testing.T) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:30073/")
	assert.Nil(t, err, "Unexpected error when dialing the AMQP connection")
	assert.NotNil(t, conn, "Connection nil, unexpected")

	defer conn.Close()
	t.Log("Connected to the amqp server")
	// creating a channel - an exchange?
	ch, err := conn.Channel()

	assert.Nil(t, err, "Unexpected error when starting a channel")
	assert.NotNil(t, ch, "Channel on the connection is nil, unexpected")

	// queue declaration
	q, err := ch.QueueDeclare("scraper_listen", false, false, false, false, nil)
	assert.Nil(t, err, "Unexpected error when declaring queue")
	assert.NotNil(t, q, "Declared queue is nil , unexpected")

	// Publishing over the channel,
	err = ch.Publish("", "scraper_listen", false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("This is yet another message"),
	})
	assert.Nil(t, err, "Unexpected error when publishing")
	t.Log("Successfully published messages on the queue")
}
