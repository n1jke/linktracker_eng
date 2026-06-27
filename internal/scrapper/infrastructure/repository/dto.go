package repository

import "github.com/n1jke/linktracker_eng/internal/scrapper/application"

type OutboxRecord struct {
	ID         int
	RetryCount int
	Shot       *application.ResourceShot
}
