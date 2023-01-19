package adyen

import (
    "context"
    "fmt"
    "net/http"
    "encoding/json"

    "github.com/netlify/gocommerce/models"
    "github.com/netlify/gocommerce/payments"
    "github.com/pkg/errors"
    "github.com/sirupsen/logrus"
    adyen "github.com/adyen/adyen-go-api-library"
)

type adyenPaymentProvider struct {
    client *adyen.Client
}

type adyenBodyParams struct {
    AdyenPaymentMethodID string `json:"adyen_payment_method_id"`
}

// Config contains the Adyen-specific configuration for payment providers.
type Config struct {
    APIKey string `mapstructure:"api_key" json:"api_key"`
}

// NewPaymentProvider creates a new Adyen payment provider using the provided configuration.
func NewPaymentProvider(config Config) (payments.Provider, error) {
    if config.APIKey == "" {
        return nil, errors.New("Adyen configuration missing api_key")
    }
    s := adyenPaymentProvider{
        client: adyen.NewClient(config.APIKey),
    }
    return &s, nil
}

func (s *adyenPaymentProvider) Name() string {
    return payments.AdyenProvider
}

func (s *adyenPaymentProvider) NewCharger(ctx context.Context, r *http.Request, log logrus.FieldLogger) (payments.Charger, error) {
    var bp adyenBodyParams
    bod, err := r.GetBody()
    if err != nil {
        return nil, err
    }
    err = json.NewDecoder(bod).Decode(&bp)
    if err != nil {
        return nil, err
    }
    if bp.AdyenPaymentMethodID == "" {
        return nil, errors.New("Adyen requires a adyen_payment_method_id for creating a payment")
    }
    return func(amount uint64, currency string, order *models.Order, invoiceNumber int64) (string, error) {
        return s.chargePayment(bp.AdyenPaymentMethodID, amount, currency, order, invoiceNumber)
    }, nil
}

func prepareShippingAddress(addr models.Address) *adyen.Address {
    return &adyen.Address{
        Street: addr.Address1,
        City: addr.City,
        StateOrProvince: addr.State,
        PostalCode: addr.Zip,
        Country:addr.Country,
    }
}
func (s *adyenPaymentProvider) chargePayment(paymentMethodID string, amount uint64, currency string, order *models.Order, invoiceNumber int64) (string, error) {
    params := &adyen.PaymentRequest{
        PaymentMethod: paymentMethodID,
        Amount: &adyen.Amount{
            Currency: currency,
            Value: fmt.Sprintf("%d", amount),
        },
        Reference: fmt.Sprintf("Invoice No. %d", invoiceNumber),
        ShippingAddress: prepareShippingAddress(order.ShippingAddress),
        AdditionalData: map[string]string{
            "order_id": order.ID,
            "invoice_number": fmt.Sprintf("%d", invoiceNumber),
        },
    }
    paymentResponse, err := s.client.Payments.Authorise(params)
    if err != nil {
        return "", err
    }
    if paymentResponse.ResultCode != "Authorised" {
        return "", fmt.Errorf("Adyen payment failed with result code: %s", paymentResponse.ResultCode)
    }
    return paymentResponse.PspReference, nil
}
