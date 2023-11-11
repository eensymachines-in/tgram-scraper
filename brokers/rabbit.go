package brokers

import (
	"fmt"

	"github.com/streadway/amqp"
)

type RabbitMQBroker struct {
	Conn  *amqp.Connection
	QName string
}

func (rbmq *RabbitMQBroker) Publish(byt []byte) error {
	if rbmq.Conn == nil {
		return fmt.Errorf("nil broker connection, cannot publish")
	}
	if byt == nil {
		return fmt.Errorf("nil/invalid message to publish, cannot continue")
	}
	// Starting a new channel
	ch, err := rbmq.Conn.Channel()
	if err != nil || ch == nil {
		return fmt.Errorf("error creating a new channel to Rabbit, %s", err)
	}
	defer ch.Close()

	// Publishing the message on the channel,. default exchange
	err = ch.Publish("", rbmq.QName, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        byt,
	})
	if err != nil {
		return fmt.Errorf("failed publishing message on rabbit channel, %s", err)
	}
	return nil
}

func (rbmq *RabbitMQBroker) DeclareQueue(name string) error {
	ch, err := rbmq.Conn.Channel()
	if err != nil || ch == nil {
		return fmt.Errorf("error creating a new channel to Rabbit, %s", err)
	}
	defer ch.Close()
	_, err = ch.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error declaring new queue: %s", err)
	}
	return nil
}
