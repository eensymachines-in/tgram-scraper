package brokers_test

import (
	"fmt"

	"github.com/eensymachines/tgramscraper/brokers"
)

func ExampleRabbitConnDial() {
	// Make a connection to the rabbit broker and then object with context lets you publish / bind a queue
	connResult, err := brokers.RabbitConnDial("guest", "guest", "localhost:30073")
	fmt.Println(connResult != nil)
	fmt.Println(err == nil)
	defer connResult.CloseConn()
	// Bind a queue, that listens on the topic for the exchange
	err = connResult.BindAQueue("listen.reader", "amq.topic", "botid.updates")
	fmt.Println(err == nil)
	// Publish a message and head over to the management console to see if the message is received on the queue
	err = connResult.Publish([]byte("Hello there from inside the sample test"), "amq.topic", "botid.updates")
	fmt.Println(err == nil)
	// Output:
	// true
	// true
	// true
	// true
}

func ExampleRabbitConnDial_multiQueueBinding() {
	connResult, _ := brokers.RabbitConnDial("guest", "guest", "localhost:30073")
	defer connResult.CloseConn()
	// Trying to bind queues with different attributes to the same exchange
	err := connResult.BindAQueue("listen.reader", "amq.topic", "botid.updates")
	fmt.Println(err == nil)
	err = connResult.BindAQueue("telegram.parser", "amq.topic", "botid.updates")
	fmt.Println(err == nil)
	// same queue can have 2 bindings ? - yes why not? if its the same consumer then why not?
	err = connResult.BindAQueue("telegram.parser", "amq.topic", "botid.logs") // tricky one .. what does it do?
	fmt.Println(err == nil)

	err = connResult.Publish([]byte("This message should be fanned out to all the queues under the topic botid.updates"), "amq.topic", "botid.updates")
	fmt.Println(err == nil)

	err = connResult.Publish([]byte("This is the log message, not an update under the topic botid.logs"), "amq.topic", "botid.logs")
	fmt.Println(err == nil)
	// Output:
	// true
	// true
	// true
	// true
	// true
}
