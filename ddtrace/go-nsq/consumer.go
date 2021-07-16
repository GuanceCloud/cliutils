package nsq

import (
	"math"
	"time"

	"github.com/nsqio/go-nsq"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Consumer struct {
	*nsq.Consumer
	*traceHelper
}

func NewConsumer(topic string, channel string, config *nsq.Config, opts ...Option) (*Consumer, error) {
	consumer, err := nsq.NewConsumer(topic, channel, config)
	if err != nil {
		return nil, err
	}

	cfg := NewConfig(opts...)
	cfg.Config = config

	return &Consumer{
		Consumer:    consumer,
		traceHelper: newTraceHelper(cfg),
	}, nil
}

func (this *Consumer) Stats() *nsq.ConsumerStats {
	start := time.Now()
	stats := this.Consumer.Stats()
	this.traceHelper.trace(start, spanTypeConsumer, "Stats", nil)

	return stats
}

func (this *Consumer) SetBehaviorDelegate(cb interface{}) {
	start := time.Now()
	this.Consumer.SetBehaviorDelegate(cb)
	this.traceHelper.trace(start, spanTypeConsumer, "SetBehaviorDelegate", nil)
}

func (this *Consumer) IsStarved() bool {
	start := time.Now()
	is := this.Consumer.IsStarved()
	this.traceHelper.trace(start, spanTypeConsumer, "IsStarved", nil)

	return is
}

func (this *Consumer) ChangeMaxInFlight(maxInFlight int) {
	start := time.Now()
	this.Consumer.ChangeMaxInFlight(maxInFlight)
	this.traceHelper.trace(start, spanTypeConsumer, "ChangeMaxInFlight", nil)
}

func (this *Consumer) ConnectToNSQLookupd(addr string) error {
	start := time.Now()
	err := this.Consumer.ConnectToNSQLookupd(addr)
	this.traceHelper.trace(start, spanTypeConsumer, "ConnectToNSQLookupd", err)

	return err
}

func (this *Consumer) ConnectToNSQLookupds(addresses []string) error {
	start := time.Now()
	err := this.Consumer.ConnectToNSQLookupds(addresses)
	this.traceHelper.trace(start, spanTypeConsumer, "ConnectToNSQLookupds", err)

	return err
}

func (this *Consumer) ConnectToNSQDs(addresses []string) error {
	start := time.Now()
	err := this.Consumer.ConnectToNSQDs(addresses)
	this.traceHelper.trace(start, spanTypeConsumer, "ConnectToNSQDs", err)

	return err
}

func (this *Consumer) ConnectToNSQD(addr string) error {
	start := time.Now()
	err := this.Consumer.ConnectToNSQD(addr)
	this.traceHelper.trace(start, spanTypeConsumer, "ConnectToNSQD", err)

	return err
}

func (this *Consumer) DisconnectFromNSQD(addr string) error {
	start := time.Now()
	err := this.Consumer.DisconnectFromNSQD(addr)
	this.traceHelper.trace(start, spanTypeConsumer, "DisconnectFromNSQD", err)

	return err
}

func (this *Consumer) DisconnectFromNSQLookupd(addr string) error {
	start := time.Now()
	err := this.Consumer.DisconnectFromNSQLookupd(addr)
	this.traceHelper.trace(start, spanTypeConsumer, "DisconnectFromNSQLookupd", err)

	return err
}

func (this *Consumer) AddHandler(handler nsq.Handler) {
	start := time.Now()
	this.Consumer.AddHandler(handler)
	this.traceHelper.trace(start, spanTypeConsumer, "AddHandler", nil)
}

func (this *Consumer) AddConcurrentHandlers(handler nsq.Handler, concurrency int) {
	start := time.Now()
	this.Consumer.AddConcurrentHandlers(handler, concurrency)
	this.traceHelper.trace(start, spanTypeConsumer, "AddConcurrentHandlers", nil)
}

func (this *Consumer) Stop() {
	start := time.Now()
	this.Consumer.Stop()
	this.traceHelper.trace(start, spanTypeConsumer, "Stop", nil)
}

func (this *Consumer) Middleware(handler nsq.Handler, resource string) nsq.Handler {
	return (nsq.HandlerFunc)(func(msg *nsq.Message) error {
		opts := []ddtrace.StartSpanOption{
			tracer.ServiceName(this.traceHelper.cfg.service),
			tracer.ResourceName(resource),
			tracer.SpanType(string(spanTypeProducer)),
		}
		if !math.IsNaN(this.traceHelper.cfg.analyticsRate) {
			opts = append(opts, tracer.Tag(ext.EventSampleRate, this.traceHelper.cfg.analyticsRate))
		}

		span, ctx := tracer.StartSpanFromContext(this.traceHelper.cfg.ctx, "Consumer.HandleMessage", opts...)
		defer span.Finish()

		this.traceHelper.cfg.ctx = ctx

		err := handler.HandleMessage(msg)
		if err != nil {
			span.SetTag("HandleMessage.Error", err)
		}

		return err
	})
}