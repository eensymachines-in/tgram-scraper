package brokers

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

// Payload sent back by RabbitConnDial that holds pointers to functions for operating on the broker further.
// Is the context object that can provide for further functions to call in the context of the given connection.
type RabbitConnResult struct {
	Publish func(message []byte, excName, topic string) error // publishing messages to exchanges
	// A queue per listener.
	// A single listener can have 2 queues bound to the same exchange, but most probably with distinct topics
	// trying to call this function with identical names for the same exchange and topic will do nothing
	// 2 or more listeners cannot have a single queue, fan in isnt allowed.
	BindAQueue    func(name, excName, topic string) error // binds a queue with a name to an exchange under a specific topic
	ListenOnQueue func(name string) (<-chan amqp.Delivery, error)
	CloseConn     func() // closes the connection
}

// RabbitConnDial is a closure around amqp.Connection, that lets you do publishing and listening on a exchange and queue
// Sends back a connection result which is set of pointers to functions to operate on further.
func RabbitConnDial(user, passwd, server string) (*RabbitConnResult, error) {
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s/", user, passwd, server))
	if err != nil {
		return nil, nil
	}
	log.WithFields(log.Fields{
		"connection_isnotnil": conn != nil,
	}).Debug("connected to rabbitmq broker")
	ch, err := conn.Channel()
	if ch == nil || err != nil {
		return nil, fmt.Errorf("failed RabbitConnDial: %s", err)
	}
	log.WithFields(log.Fields{
		"channel_isnotnil": conn != nil,
	}).Debug("established channel to rabbitmq broker")
	return &RabbitConnResult{
		Publish: func(message []byte, excName, topic string) error {
			return ch.Publish(excName, topic, false, false, amqp.Publishing{
				ContentType: "text/plain",
				Body:        message,
			})
		},
		BindAQueue: func(name, excName, topic string) error {
			// Binding 2 queues with the same name to the same exchange subscribing to the same topic will do nothing.
			// Will NOT create a new queue.
			// When only the name differs 2 queues can have bound to the same excahnge under the samep topic.
			_, err := ch.QueueDeclare(name, false, false, false, false, nil)
			if err != nil {
				return err
			}
			return ch.QueueBind(name, topic, excName, false, nil)
		},
		ListenOnQueue: func(name string) (<-chan amqp.Delivery, error) {
			return ch.Consume(name, "", true, false, false, false, nil)
		},
		CloseConn: func() {
			ch.Close()
			conn.Close()
		},
	}, nil
}
