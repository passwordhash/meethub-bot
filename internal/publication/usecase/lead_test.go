package usecase

import (
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func TestSelectLead(t *testing.T) {
	tests := []struct {
		name           string
		reports        []models.Report
		previousLeadID int64
		wantNil        bool
		wantIDs        []int64
	}{
		{name: "no reporters", previousLeadID: 1, wantNil: true},
		{name: "only reporter may lead again", reports: []models.Report{{AuthorID: 1, Author: "one"}}, previousLeadID: 1, wantIDs: []int64{1}},
		{name: "previous lead excluded with alternatives", reports: []models.Report{{AuthorID: 1}, {AuthorID: 2}, {AuthorID: 3}}, previousLeadID: 1, wantIDs: []int64{2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, err := SelectLead(tt.reports, tt.previousLeadID)
			if err != nil {
				t.Fatalf("SelectLead() error = %v", err)
			}
			if tt.wantNil {
				if selected != nil {
					t.Fatalf("SelectLead() = %#v, want nil", selected)
				}
				return
			}
			if selected == nil {
				t.Fatal("SelectLead() = nil, want a reporter")
			}
			for _, id := range tt.wantIDs {
				if selected.AuthorID == id {
					return
				}
			}
			t.Fatalf("selected author ID %d not in %v", selected.AuthorID, tt.wantIDs)
		})
	}
}
