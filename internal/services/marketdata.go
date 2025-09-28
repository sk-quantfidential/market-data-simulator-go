package services

import (
	"github.com/sirupsen/logrus"

	"github.com/quantfidential/trading-ecosystem/market-data-simulator-go/internal/config"
)

type MarketDataService struct {
	config *config.Config
	logger *logrus.Logger
}

func NewMarketDataService(cfg *config.Config, logger *logrus.Logger) *MarketDataService {
	return &MarketDataService{
		config: cfg,
		logger: logger,
	}
}

func (s *MarketDataService) GetPrice(symbol string) (float64, error) {
	s.logger.WithField("symbol", symbol).Info("Getting price for symbol")
	return 100.0, nil
}

func (s *MarketDataService) Subscribe(symbol string) error {
	s.logger.WithField("symbol", symbol).Info("Subscribing to symbol")
	return nil
}