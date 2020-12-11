package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/mb-14/rzp-spotlight/webhook/json"
)

const (
	DBName = "rzpftx"
)

var (
	client   = influxdb2.NewClientWithOptions("http://localhost:8086", "", influxdb2.DefaultOptions().SetBatchSize(2000))
	writeAPI = client.WriteAPI("", DBName)
)

// Events
const (
	PaymentAuthorized = "payment.authorized"
	PaymentFailed     = "payment.failed"
)

// Methods
const (
	Netbanking = "netbanking"
	Wallet     = "wallet"
	UPI        = "upi"
	Card       = "card"
)

func webhookEventHandler(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	json := json.Json{Data: payload}
	var event string
	if value, err := json.GetString("event"); err == nil {
		event = value
	}
	if event == PaymentAuthorized {
		err = writePaymentEvent(c.Request.Context(), json, "payment_authorized")
	} else if event == PaymentFailed {
		err = writePaymentEvent(c.Request.Context(), json, "payment_failed")
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})

}

func writePaymentEvent(ctx context.Context, json json.Json, event string) error {
	amount, _ := json.GetInt("payload.payment.entity.amount")
	createdAt, _ := json.GetTime("payload.payment.entity.created_at")
	p := influxdb2.NewPoint(event,
		addTags(json),
		map[string]interface{}{"amount": amount},
		createdAt)
	writeAPI.WritePoint(p)
	return nil
}

func addTags(p json.Json) map[string]string {
	tags := make(map[string]string)
	// Common tags
	method, _ := p.GetString("payload.payment.entity.method")
	currency, _ := p.GetString("payload.payment.entity.currency")
	tags["currency"] = currency
	tags["method"] = method
	if method == Netbanking {
		tags["bank"], _ = p.GetString("payload.payment.entity.bank")
	}

	if method == Wallet {
		tags["walletName"], _ = p.GetString("payload.payment.entity.wallet")
	}

	if method == UPI {
		vpa, _ := p.GetString("payload.payment.entity.vpa")
		vpaString := strings.Split(vpa, "@")
		tags["upiPsp"] = vpaString[1]
	}

	if method == Card {
		tags["cardNetwork"], _ = p.GetString("payload.payment.entity.card.network")
		tags["cardType"], _ = p.GetString("payload.payment.entity.card.type")
		tags["cardInternational"], _ = p.GetString("payload.payment.entity.card.international")
		tags["cardIssuer"], _ = p.GetString("payload.payment.entity.card.issuer")
	}

	return tags
}
