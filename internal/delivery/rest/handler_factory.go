package rest

import (
	"sub-balance-implementation/internal/infra/postgres"
	"sub-balance-implementation/internal/usecase"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// HandlerFactory creates and manages all handler instances
type HandlerFactory struct {
	db     *gorm.DB
	logger *zap.Logger
	repos  *postgres.AllRepositories
	usecases *usecase.AllUsecases
}

// NewHandlerFactory creates a new handler factory
func NewHandlerFactory(db *gorm.DB, logger *zap.Logger) *HandlerFactory {
	repoFactory := postgres.NewRepositoryFactory(db, logger)
	repos := repoFactory.GetAllRepositories()
	
	usecaseFactory := usecase.NewUsecaseFactory(repos, logger)
	usecases := usecaseFactory.GetAllUsecases()
	
	return &HandlerFactory{
		db:       db,
		logger:   logger,
		repos:    repos,
		usecases: usecases,
	}
}

// GetHandler returns the REST handler
func (hf *HandlerFactory) GetHandler() *Handler {
	return &Handler{
		db:       hf.db,
		logger:   hf.logger,
		repos:    hf.repos,
		usecases: hf.usecases,
	}
}

// GetRepositories returns all repositories
func (hf *HandlerFactory) GetRepositories() *postgres.AllRepositories {
	return hf.repos
}

// GetUsecases returns all usecases
func (hf *HandlerFactory) GetUsecases() *usecase.AllUsecases {
	return hf.usecases
}
