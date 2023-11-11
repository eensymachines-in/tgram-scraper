package brokers

type Broker interface {
	Publish([]byte) error
}

// Brokers that need queues, need this interface to perform q operations
type QueuedBroker interface {
	DeclareQueue(name string) error
}
