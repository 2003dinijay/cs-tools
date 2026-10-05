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

package main

import "testing"

var (
	crT        = consumerTarget{"cr-events", "csm-notification-service-cr"}
	crDLQT     = consumerTarget{"cr-events-dlq", "csm-notification-service-cr-dlq"}
	outageDef  = consumerTarget{"outage-events", "csm-notification-service-outage"}
	outageDLQT = consumerTarget{"outage-events-dlq", "csm-notification-service-outage-dlq"}
)

// Not configured: every existing consumer runs, nothing new starts.
func TestPlanSREConsumers_UnsetChangesNothing(t *testing.T) {
	p := planSREConsumers("", "", "", "", crT, crDLQT, outageDef, outageDLQT)
	if p.Enabled || !p.StartCR || !p.StartCRDLQ || !p.StartOutage || !p.StartOutageDLQ {
		t.Fatalf("unset SRE topic must leave every consumer as it was: %+v", p)
	}
}

// *** STAGING TODAY. *** The outage consumer already reads sre-events
// (OUTAGE_EVENT_HUB_TOPIC=sre-events). The SRE consumer must take over that
// group, or a fresh group starting at the first offset would re-send every
// outage email still retained in the topic -- and the outage consumer must
// stop, or every email goes out twice.
func TestPlanSREConsumers_TakesOverTheGroupAlreadyReadingTheTopic(t *testing.T) {
	outageOnSRE := consumerTarget{"sre-events", "csm-notification-service-outage"}
	p := planSREConsumers("sre-events", "", "", "", crT, crDLQT, outageOnSRE, outageDLQT)
	if !p.Enabled || p.SRE.Topic != "sre-events" || p.SRE.Group != "csm-notification-service-outage" {
		t.Fatalf("SRE consumer = %+v, want sre-events read by the outage group it replaces", p.SRE)
	}
	if p.StartOutage {
		t.Error("the outage consumer still starts on the same topic: every outage email would be sent twice")
	}
	if !p.StartCR || !p.StartCRDLQ {
		t.Error("the CR consumer on cr-events must keep running to drain what is already queued there")
	}
	if p.SREDLQ.Topic != "sre-events-dlq" || p.SREDLQ.Group != defaultSREDLQConsumerGroup {
		t.Errorf("SRE DLQ = %+v, want sre-events-dlq with the SRE DLQ group", p.SREDLQ)
	}
}

// A fresh topic nobody reads yet gets the SRE group, starting from the
// beginning -- which on a new topic is everything it should see.
func TestPlanSREConsumers_FreshTopicUsesTheSREGroup(t *testing.T) {
	p := planSREConsumers("sre-events", "", "", "", crT, crDLQT, outageDef, outageDLQT)
	if p.SRE.Group != defaultSREConsumerGroup {
		t.Errorf("group = %q, want %q", p.SRE.Group, defaultSREConsumerGroup)
	}
	if !p.StartCR || !p.StartOutage {
		t.Error("consumers on other topics must keep running while producers switch over")
	}
}

func TestPlanSREConsumers_ExplicitGroupAndSharedDLQ(t *testing.T) {
	p := planSREConsumers("sre-events", "my-group", "outage-events-dlq", "", crT, crDLQT, outageDef, outageDLQT)
	if p.SRE.Group != "my-group" {
		t.Errorf("an explicit SRE_CONSUMER_GROUP must win, got %q", p.SRE.Group)
	}
	if p.SREDLQ.Group != "csm-notification-service-outage-dlq" || p.StartOutageDLQ {
		t.Errorf("sharing the outage DLQ topic must reuse its group and stop its own DLQ consumer: %+v start=%v", p.SREDLQ, p.StartOutageDLQ)
	}
}
