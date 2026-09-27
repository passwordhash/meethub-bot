package usecase

import (
	"crypto/rand"
	"math/big"

	reportdomain "github.com/passwordhash/cascade-hub-bot/internal/report/domain"
	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func SelectLead(reports []models.Report, previousLeadID int64) (*models.Report, error) {
	eligibleIDs := reportdomain.EligibleLeadIDs(reports, previousLeadID)
	if len(eligibleIDs) == 0 {
		return nil, nil
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(eligibleIDs))))
	if err != nil {
		return nil, err
	}
	selectedID := eligibleIDs[index.Int64()]
	for index := range reports {
		if reports[index].AuthorID == selectedID {
			return &reports[index], nil
		}
	}
	return nil, nil
}
