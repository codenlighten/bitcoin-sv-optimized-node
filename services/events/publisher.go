package events

import (
	"context"
	"fmt"
	"log"
	"time"

	eventsv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/events/v1"
	"google.golang.org/protobuf/proto"
)

// Publisher interface for event publishing
type Publisher interface {
	Publish(ctx context.Context, topic string, event *eventsv1.Envelope) error
	Close() error
}

// Consumer interface for event consumption
type Consumer interface {
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	Close() error
}

// EventHandler processes incoming events
type EventHandler func(ctx context.Context, event *eventsv1.Envelope) error

// InMemoryPublisher is a simple in-memory event bus for development
type InMemoryPublisher struct {
	subscribers map[string][]EventHandler
	buffer      chan *topicEvent
}

type topicEvent struct {
	topic string
	event *eventsv1.Envelope
}

func NewInMemoryPublisher() *InMemoryPublisher {
	p := &InMemoryPublisher{
		subscribers: make(map[string][]EventHandler),
		buffer:      make(chan *topicEvent, 1000),
	}
	
	// Start event processing goroutine
	go p.processEvents()
	
	return p
}

func (p *InMemoryPublisher) Publish(ctx context.Context, topic string, event *eventsv1.Envelope) error {
	select {
	case p.buffer <- &topicEvent{topic: topic, event: event}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("event buffer full")
	}
}

func (p *InMemoryPublisher) Subscribe(topic string, handler EventHandler) {
	p.subscribers[topic] = append(p.subscribers[topic], handler)
}

func (p *InMemoryPublisher) processEvents() {
	for topicEvent := range p.buffer {
		handlers, exists := p.subscribers[topicEvent.topic]
		if !exists {
			continue
		}
		
		for _, handler := range handlers {
			go func(h EventHandler, e *eventsv1.Envelope) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				
				if err := h(ctx, e); err != nil {
					log.Printf("Event handler error for topic %s: %v", topicEvent.topic, err)
				}
			}(handler, topicEvent.event)
		}
	}
}

func (p *InMemoryPublisher) Close() error {
	close(p.buffer)
	return nil
}

// EventBuilder helps create properly formatted events
type EventBuilder struct {
	net string
}

func NewEventBuilder(net string) *EventBuilder {
	return &EventBuilder{net: net}
}

func (b *EventBuilder) CreateEnvelope(traceId string, payload proto.Message) (*eventsv1.Envelope, error) {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	return &eventsv1.Envelope{
		MsgId:     generateULID(),
		EmittedAt: time.Now().UTC().Format(time.RFC3339),
		TraceId:   traceId,
		Net:       b.net,
		Payload:   payloadBytes,
	}, nil
}

func (b *EventBuilder) CreateRawTxEvent(txBytes []byte, peerId string) (*eventsv1.RawTx, error) {
	return &eventsv1.RawTx{
		Tx:     txBytes,
		PeerId: peerId,
		SeeAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (b *EventBuilder) CreateTxValidatedEvent(txid []byte, size, sigChecks uint32) (*eventsv1.TxValidated, error) {
	return &eventsv1.TxValidated{
		Txid:        txid,
		Size:        size,
		SigChecks:   sigChecks,
		ValidatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (b *EventBuilder) CreateTxReadyEvent(txid []byte, feeRate float64, ancestors, weight uint32, priority string) (*eventsv1.TxReady, error) {
	return &eventsv1.TxReady{
		Txid:      txid,
		FeeRate:   feeRate,
		Ancestors: ancestors,
		Weight:    weight,
		Priority:  priority,
	}, nil
}

// Simple ULID generator for message IDs
func generateULID() string {
	// In production, use a proper ULID library
	// For now, use timestamp + random suffix
	return fmt.Sprintf("%d_%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000000)
}

// EventLogger provides structured logging for events
type EventLogger struct {
	publisher Publisher
	net       string
}

func NewEventLogger(publisher Publisher, net string) *EventLogger {
	return &EventLogger{
		publisher: publisher,
		net:       net,
	}
}

func (l *EventLogger) LogTxValidated(ctx context.Context, txid []byte, size, sigChecks uint32, traceId string) error {
	builder := NewEventBuilder(l.net)
	
	payload, err := builder.CreateTxValidatedEvent(txid, size, sigChecks)
	if err != nil {
		return err
	}
	
	envelope, err := builder.CreateEnvelope(traceId, payload)
	if err != nil {
		return err
	}
	
	return l.publisher.Publish(ctx, "tx.validated.v1", envelope)
}

func (l *EventLogger) LogTxReady(ctx context.Context, txid []byte, feeRate float64, ancestors, weight uint32, priority, traceId string) error {
	builder := NewEventBuilder(l.net)
	
	payload, err := builder.CreateTxReadyEvent(txid, feeRate, ancestors, weight, priority)
	if err != nil {
		return err
	}
	
	envelope, err := builder.CreateEnvelope(traceId, payload)
	if err != nil {
		return err
	}
	
	return l.publisher.Publish(ctx, "tx.ready.v1", envelope)
}
