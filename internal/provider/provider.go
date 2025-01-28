// Package provider handles data fetching from external sources
package provider

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// Config represents the configuration for a provider
type Config struct {
	// Schedule in cron format (e.g. "*/15 * * * *" for every 15 minutes)
	Schedule string `json:"schedule"`
	// Enabled determines if the provider should run on schedule
	Enabled bool `json:"enabled"`
	// SupportedZones is a list of zone names that this provider supports
	SupportedZones []string `json:"supported_zones"`
	// SupportedCurrencies is a list of currency codes that this provider supports
	SupportedCurrencies []string `json:"supported_currencies"`
}

// RunOptions represents the options for a manual provider run
type RunOptions struct {
	Date     time.Time
	Zone     string
	Currency string
}

// Provider is the interface that all data providers must implement
type Provider interface {
	// Name returns the unique name of the provider
	Name() string
	// Run executes the provider's data fetching and storing logic
	Run(ctx context.Context) error
	// RunWithOptions executes the provider with specific options (for manual runs)
	RunWithOptions(ctx context.Context, opts RunOptions) error
	// GetConfig returns the provider's configuration
	GetConfig() Config
	// SupportsZone checks if the provider supports a given zone
	SupportsZone(zoneName string) bool
	// SupportsCurrency checks if the provider supports a given currency
	SupportsCurrency(currencyCode string) bool
}

// BaseProvider contains common functionality for all providers
type BaseProvider struct {
	db     *sql.DB
	config Config
}

// NewBaseProvider creates a new BaseProvider
func NewBaseProvider(db *sql.DB, config Config) BaseProvider {
	return BaseProvider{
		db:     db,
		config: config,
	}
}

// GetConfig returns the provider's configuration
func (p *BaseProvider) GetConfig() Config {
	return p.config
}

// SupportsZone checks if the provider supports a given zone
func (p *BaseProvider) SupportsZone(zoneName string) bool {
	for _, zone := range p.config.SupportedZones {
		if zone == zoneName {
			return true
		}
	}
	return false
}

// SupportsCurrency checks if the provider supports a given currency
func (p *BaseProvider) SupportsCurrency(currencyCode string) bool {
	for _, currency := range p.config.SupportedCurrencies {
		if currency == currencyCode {
			return true
		}
	}
	return false
}

// GetDB returns the database connection
func (p *BaseProvider) GetDB() *sql.DB {
	return p.db
}

// Manager handles the scheduling and execution of providers
type Manager struct {
	providers []Provider
	db        *sql.DB
	cron      *cron.Cron
}

// NewManager creates a new provider manager
func NewManager(db *sql.DB) *Manager {
	// Create a new cron scheduler with seconds disabled
	c := cron.New(cron.WithParser(cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)))

	return &Manager{
		db:        db,
		providers: make([]Provider, 0),
		cron:      c,
	}
}

// RegisterProvider adds a provider to the manager
func (m *Manager) RegisterProvider(p Provider) {
	m.providers = append(m.providers, p)
}

// GetProvider returns a provider by name
func (m *Manager) GetProvider(name string) (Provider, bool) {
	for _, p := range m.providers {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

// RunProvider executes a specific provider by name
func (m *Manager) RunProvider(ctx context.Context, name string, opts *RunOptions) error {
	provider, found := m.GetProvider(name)
	if !found {
		return ErrProviderNotFound
	}

	// Check if provider is enabled
	if !provider.GetConfig().Enabled {
		return fmt.Errorf("provider %s is disabled", name)
	}

	if opts != nil {
		// Validate options
		if !provider.SupportsZone(opts.Zone) {
			return fmt.Errorf("provider %s does not support zone %s", name, opts.Zone)
		}
		if !provider.SupportsCurrency(opts.Currency) {
			return fmt.Errorf("provider %s does not support currency %s", name, opts.Currency)
		}
		return provider.RunWithOptions(ctx, *opts)
	}

	return provider.Run(ctx)
}

// StartScheduler starts all enabled providers on their configured schedules
func (m *Manager) StartScheduler(ctx context.Context) error {
	for _, p := range m.providers {
		config := p.GetConfig()
		if !config.Enabled {
			log.Printf("Provider %s is disabled, skipping scheduler", p.Name())
			continue
		}

		if config.Schedule == "" {
			return fmt.Errorf("provider %s has no schedule configured", p.Name())
		}

		// Create a closure to capture the provider
		provider := p
		_, err := m.cron.AddFunc(config.Schedule, func() {
			log.Printf("Running scheduled execution of provider %s", provider.Name())
			if err := provider.Run(ctx); err != nil {
				log.Printf("Error running provider %s: %v", provider.Name(), err)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to schedule provider %s: %w", p.Name(), err)
		}

		log.Printf("Scheduled provider %s with schedule %s", p.Name(), config.Schedule)
	}

	// Start the cron scheduler
	m.cron.Start()
	log.Println("Provider scheduler started")

	// Wait for context cancellation
	<-ctx.Done()
	log.Println("Stopping provider scheduler...")
	m.cron.Stop()

	return nil
}
