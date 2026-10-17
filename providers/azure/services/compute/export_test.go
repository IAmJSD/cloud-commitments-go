package compute

import "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"

// SetExchangeClientOptionsForTest points the real CalculateExchange and
// Exchange SDK clients at caller-supplied ARM options (a fake transport), so
// tests exercise the production LRO wrappers without reaching Azure.
func (c *Client) SetExchangeClientOptionsForTest(opts *arm.ClientOptions) {
	c.exchangeClientOptions = opts
}
