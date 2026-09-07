// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package eventbus wraps github.com/segmentio/kafka-go to talk to Azure
// Event Hub's Kafka-compatible endpoint. It provides just two things: a
// Producer (publish a record, wait for the broker's ack) and a Consumer (join
// a consumer group, poll records, commit offsets after they're handled).
// Everything else — event schema, dispatch-by-type, retries — lives in the
// events and dispatch packages; this package only knows about bytes (see
// Record), so no other package needs to import the underlying Kafka client
// library.
package eventbus

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Config holds the connection settings shared by Producer and Consumer.
type Config struct {
	// Broker is the Kafka-compatible bootstrap address, e.g.
	// "<namespace>.servicebus.windows.net:9093" — the standard Kafka port
	// Event Hub's Standard tier and above expose alongside its native AMQP
	// endpoint.
	Broker string
	// ConnectionString is the Event Hub namespace's Shared Access Policy
	// connection string (Namespace > Shared access policies > a policy's
	// Primary Connection String). This is the SASL/PLAIN password; Event
	// Hub's Kafka surface always expects the literal username
	// "$ConnectionString" — see saslMechanism.
	ConnectionString string
	// Topic is the Event Hub name (Kafka topic) to produce to / consume
	// from.
	Topic string
}

// saslMechanism builds the SASL/PLAIN credential Event Hub's Kafka endpoint
// requires: username is always the literal string "$ConnectionString" (not a
// real username — this tells Event Hub the password is a connection string,
// not an Azure AD token), and the password is the connection string itself.
func (c Config) saslMechanism() sasl.Mechanism {
	return plain.Mechanism{
		Username: "$ConnectionString",
		Password: c.ConnectionString,
	}
}

// partitionCountProbeTimeout bounds PartitionCount's entire dial+read —
// without it, an unreachable broker would block startConsumers (and
// therefore the whole service's startup) indefinitely, since the ctx main.go
// passes in has no deadline of its own (it only cancels on SIGINT/SIGTERM).
const partitionCountProbeTimeout = 10 * time.Second

// PartitionCount returns cfg.Topic's current partition count — used at
// startup to sanity-check a configured consumer count against reality (see
// cmd/server/main.go): a Kafka consumer group only ever hands out as many
// partitions as exist, so a consumer count higher than this leaves some
// consumers permanently idle rather than doing anything wrong. Not used for
// anything at runtime beyond that one startup check.
func PartitionCount(ctx context.Context, cfg Config) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, partitionCountProbeTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline() // always ok immediately after WithTimeout

	dialer := &kafka.Dialer{
		TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
		SASLMechanism: cfg.saslMechanism(),
	}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Broker)
	if err != nil {
		return 0, fmt.Errorf("eventbus: dial %s: %w", cfg.Broker, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// DialContext's own deadline only bounds the dial above — ReadPartitions
	// takes no context and does its own network I/O, so it needs the
	// connection's read deadline set explicitly to stay bounded too.
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, fmt.Errorf("eventbus: set deadline for topic %s: %w", cfg.Topic, err)
	}

	partitions, err := conn.ReadPartitions(cfg.Topic)
	if err != nil {
		return 0, fmt.Errorf("eventbus: read partitions for topic %s: %w", cfg.Topic, err)
	}
	return len(partitions), nil
}
