package services

import (
	"github.com/sirupsen/logrus"
)

type MarketDataService struct {
	logger *logrus.Logger
}

func NewMarketDataService(logger *logrus.Logger) *MarketDataService {
	return &MarketDataService{
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