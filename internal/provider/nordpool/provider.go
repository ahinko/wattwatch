package nordpool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"wattwatch/internal/provider"
)

const (
	// ProviderName is the unique identifier for the Nordpool provider
	ProviderName = "nordpool"
	// BaseURL is the base URL for the Nordpool API
	BaseURL = "https://dataportal-api.nordpoolgroup.com/api/DayAheadPrices"
)

// MultiAreaEntry represents a single time entry with prices for multiple areas
type MultiAreaEntry struct {
	DeliveryStart time.Time          `json:"deliveryStart"`
	DeliveryEnd   time.Time          `json:"deliveryEnd"`
	EntryPerArea  map[string]float64 `json:"entryPerArea"`
}

// Response represents the response from the Nordpool API
type Response struct {
	DeliveryDateCET  string           `json:"deliveryDateCET"`
	Version          int              `json:"version"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	DeliveryAreas    []string         `json:"deliveryAreas"`
	Market           string           `json:"market"`
	MultiAreaEntries []MultiAreaEntry `json:"multiAreaEntries"`
	Currency         string           `json:"currency"`
	ExchangeRate     float64          `json:"exchangeRate"`
}

// DefaultConfig returns the default configuration for the Nordpool provider
func DefaultConfig() provider.Config {
	return provider.Config{
		Schedule: "15 12 * * *", // Run at 12:15 every day
		Enabled:  true,
		SupportedZones: []string{
			"SE1", "SE2", "SE3", "SE4", // Swedish price areas
		},
		SupportedCurrencies: []string{
			"EUR", "SEK", // Euro is base currency, SEK is local currency
		},
	}
}

// Provider implements the provider.Provider interface for Nordpool
type Provider struct {
	provider.BaseProvider
	client *http.Client
}

// NewProvider creates a new Nordpool provider
func NewProvider(db *sql.DB, config provider.Config) *Provider {
	// Merge with default config if needed
	if len(config.SupportedZones) == 0 {
		config.SupportedZones = DefaultConfig().SupportedZones
	}
	if len(config.SupportedCurrencies) == 0 {
		config.SupportedCurrencies = DefaultConfig().SupportedCurrencies
	}
	if config.Schedule == "" {
		config.Schedule = DefaultConfig().Schedule
	}

	return &Provider{
		BaseProvider: provider.NewBaseProvider(db, config),
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// Name returns the provider's unique identifier
func (p *Provider) Name() string {
	return ProviderName
}

// parsePrice converts a price by dividing by 10
func (p *Provider) parsePrice(price float64) float64 {
	return price / 10
}

// fetchPrices fetches spot prices from the Nordpool API for a specific zone and currency
func (p *Provider) fetchPrices(ctx context.Context, date time.Time, zone, currency string) ([]MultiAreaEntry, error) {
	// Build query parameters
	params := url.Values{}
	params.Add("market", "DayAhead")
	params.Add("deliveryArea", zone)
	params.Add("currency", currency)
	params.Add("date", date.Format("2006-01-02"))

	// Build request URL
	reqURL := fmt.Sprintf("%s?%s", BaseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.MultiAreaEntries, nil
}

// getZoneID fetches the ID for a given zone name from the database
func (p *Provider) getZoneID(ctx context.Context, zoneName string) (string, error) {
	var id string
	err := p.BaseProvider.GetDB().QueryRowContext(ctx,
		"SELECT id FROM zones WHERE name = $1",
		zoneName,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to fetch zone ID for %s: %w", zoneName, err)
	}
	return id, nil
}

// getCurrencyID fetches the ID for a given currency code from the database
func (p *Provider) getCurrencyID(ctx context.Context, currencyCode string) (string, error) {
	var id string
	err := p.BaseProvider.GetDB().QueryRowContext(ctx,
		"SELECT id FROM currencies WHERE name = $1",
		currencyCode,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to fetch currency ID for %s: %w", currencyCode, err)
	}
	return id, nil
}

// storePrices stores spot prices in the database
func (p *Provider) storePrices(ctx context.Context, entries []MultiAreaEntry, zoneName, currencyCode string) error {
	// Get zone and currency IDs
	zoneID, err := p.getZoneID(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("failed to get zone ID: %w", err)
	}

	currencyID, err := p.getCurrencyID(ctx, currencyCode)
	if err != nil {
		return fmt.Errorf("failed to get currency ID: %w", err)
	}

	// Start transaction
	tx, err := p.BaseProvider.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.PrepareContext(ctx, `
		WITH tz AS (
			SELECT timezone FROM zones WHERE id = $2
		)
		INSERT INTO spot_prices (timestamp, zone_id, currency_id, price)
		VALUES (
			timezone(
				(SELECT timezone FROM tz),
				$1::timestamptz
			),
			$2, $3, $4
		)
		ON CONFLICT (timestamp, zone_id, currency_id) DO UPDATE
		SET price = EXCLUDED.price
		WHERE spot_prices.price != EXCLUDED.price
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert prices
	for _, entry := range entries {
		// Get and parse price for the zone
		price, ok := entry.EntryPerArea[zoneName]
		if !ok {
			return fmt.Errorf("no price found for zone %s", zoneName)
		}

		// Convert price (divide by 10)
		price = p.parsePrice(price)

		if _, err := stmt.ExecContext(ctx, entry.DeliveryStart, zoneID, currencyID, price); err != nil {
			return fmt.Errorf("failed to insert price: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Run executes the provider's data fetching and storing logic for all supported combinations
func (p *Provider) Run(ctx context.Context) error {
	// Use tomorrow's date for scheduled runs
	tomorrow := time.Now().AddDate(0, 0, 1)

	// Fetch and store prices for all zone/currency combinations
	for _, zone := range p.GetConfig().SupportedZones {
		for _, currency := range p.GetConfig().SupportedCurrencies {
			// Add delay between API calls
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}

			entries, err := p.fetchPrices(ctx, tomorrow, zone, currency)
			if err != nil {
				return fmt.Errorf("failed to fetch prices for %s/%s: %w", zone, currency, err)
			}

			if err := p.storePrices(ctx, entries, zone, currency); err != nil {
				return fmt.Errorf("failed to store prices for %s/%s: %w", zone, currency, err)
			}
		}
	}

	return nil
}

// RunWithOptions executes the provider with specific options (for manual runs)
func (p *Provider) RunWithOptions(ctx context.Context, opts provider.RunOptions) error {
	// Validate options
	if !p.SupportsZone(opts.Zone) {
		return fmt.Errorf("unsupported zone: %s", opts.Zone)
	}
	if !p.SupportsCurrency(opts.Currency) {
		return fmt.Errorf("unsupported currency: %s", opts.Currency)
	}

	// Add delay before API call
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
	}

	// Fetch prices for the specified combination
	entries, err := p.fetchPrices(ctx, opts.Date, opts.Zone, opts.Currency)
	if err != nil {
		return fmt.Errorf("failed to fetch prices: %w", err)
	}

	// Store the prices
	if err := p.storePrices(ctx, entries, opts.Zone, opts.Currency); err != nil {
		return fmt.Errorf("failed to store prices: %w", err)
	}

	return nil
}
