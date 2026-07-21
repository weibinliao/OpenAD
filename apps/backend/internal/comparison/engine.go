package comparison

import (
	"crypto/md5"
	"fmt"
	"time"

	"github.com/weibinliao/OpenAD/internal/models"

	"github.com/google/uuid"
)

type ComparisonEngine struct {
	baseline *models.ScanSession
	current  *models.ScanSession
}

func NewComparisonEngine(baseline, current *models.ScanSession) *ComparisonEngine {
	return &ComparisonEngine{
		baseline: baseline,
		current:  current,
	}
}

func (c *ComparisonEngine) DetectChanges(baselinePerms, currentPerms []models.Permission) (*ChangeReport, error) {
	baselineMap := c.createPermissionMap(baselinePerms)
	currentMap := c.createPermissionMap(currentPerms)

	var changes []models.PermissionChange

	// Detect added permissions
	for key, perm := range currentMap {
		if _, exists := baselineMap[key]; !exists {
			changes = append(changes, models.PermissionChange{
				ID:            uuid.New(),
				ScanSessionID: c.current.ID,
				ChangeType:    "added",
				Path:          perm.Path,
				Trustee:       perm.Trustee,
				TrusteeSID:    perm.TrusteeSID,
				CurrentRights: perm.Rights,
				DetectedAt:    time.Now(),
			})
		}
	}

	// Detect removed permissions
	for key, perm := range baselineMap {
		if _, exists := currentMap[key]; !exists {
			changes = append(changes, models.PermissionChange{
				ID:             uuid.New(),
				ScanSessionID:  c.current.ID,
				ChangeType:     "removed",
				Path:           perm.Path,
				Trustee:        perm.Trustee,
				TrusteeSID:     perm.TrusteeSID,
				PreviousRights: perm.Rights,
				DetectedAt:     time.Now(),
			})
		}
	}

	// Detect modified permissions
	for key, currentPerm := range currentMap {
		if baselinePerm, exists := baselineMap[key]; exists {
			if currentPerm.Rights != baselinePerm.Rights {
				changes = append(changes, models.PermissionChange{
					ID:             uuid.New(),
					ScanSessionID:  c.current.ID,
					ChangeType:     "modified",
					Path:           currentPerm.Path,
					Trustee:        currentPerm.Trustee,
					TrusteeSID:     currentPerm.TrusteeSID,
					PreviousRights: baselinePerm.Rights,
					CurrentRights:  currentPerm.Rights,
					DetectedAt:     time.Now(),
				})
			}
		}
	}

	return &ChangeReport{
		BaselineID:   c.baseline.ID.String(),
		CurrentID:    c.current.ID.String(),
		Changes:      changes,
		ChangesCount: len(changes),
		GeneratedAt:  time.Now(),
	}, nil
}

func (c *ComparisonEngine) createPermissionMap(permissions []models.Permission) map[string]models.Permission {
	permMap := make(map[string]models.Permission)

	for _, perm := range permissions {
		key := c.createPermissionKey(perm.Path, perm.TrusteeSID)
		permMap[key] = perm
	}

	return permMap
}

func (c *ComparisonEngine) createPermissionKey(path, trusteeSID string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(path+"|"+trusteeSID)))
}

type ChangeReport struct {
	BaselineID   string                    `json:"baseline_id"`
	CurrentID    string                    `json:"current_id"`
	Changes      []models.PermissionChange `json:"changes"`
	ChangesCount int                       `json:"changes_count"`
	GeneratedAt  time.Time                 `json:"generated_at"`
}
